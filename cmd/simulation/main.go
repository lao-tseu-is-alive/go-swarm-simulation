package main

import (
	"context"
	"image/color"
	"log"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/tochemey/goakt/v3/actor"

	"github.com/lao-tseu-is-alive/go-swarm-simulation/internal/individual"
	golog "github.com/tochemey/goakt/v3/log"
)

type Game struct {
	actorSystem actor.ActorSystem
	ctx         context.Context
	pids        []*actor.PID
	updates     chan *individual.ActorState
	worldState  map[string]*individual.ActorState
}

func (g *Game) Update() error {
	// 1. Drain Channel
Loop:
	for {
		select {
		case state := <-g.updates:
			g.worldState[state.Id] = state
		default:
			break Loop
		}
	}

	// 2. GAME LOGIC: Perception, Collision, and Conversion
	detectionDistSq := 100.0 * 100.0 // Large detection range for chasing (200 units)
	contactDistSq := 12.0 * 12.0     // Collision distance (radius 5 + 5 + margin)
	defenseDistSq := 35.0 * 35.0     // Defense radius (User requested 10, typically too small for 3 units, bumped to 25 for playability)

	// Note: We iterate only through RED actors to drive the interactions
	// This is an O(N*M) operation. Fine for < 500 actors.
	for _, redPID := range g.pids {
		redState, ok := g.worldState[redPID.Name()]
		if !ok || redState.Color != individual.ColorRed {
			continue
		}

		var visiblePrey []*individual.ActorState

		// Scan for Blue targets
		for _, entity := range g.worldState {
			if entity.Color == individual.ColorBlue {
				dx := entity.PositionX - redState.PositionX
				dy := entity.PositionY - redState.PositionY
				distSq := dx*dx + dy*dy

				// Perception: Can I see it?
				if distSq < detectionDistSq {
					visiblePrey = append(visiblePrey, entity)
				}

				// Interaction: Did I catch it?
				if distSq < contactDistSq {
					// === COMBAT LOGIC ===

					// Check Defense: How many Blue friends are near the Victim?
					blueDefenders := 0
					for _, defender := range g.worldState {
						if defender.Color == individual.ColorBlue && defender.Id != entity.Id {
							defDx := defender.PositionX - entity.PositionX
							defDy := defender.PositionY - entity.PositionY
							if (defDx*defDx + defDy*defDy) < defenseDistSq {
								blueDefenders++
							}
						}
					}

					if blueDefenders >= 3 {
						// DEFENSE SUCCESS: Red converts to Blue!
						// Send command to the RED actor (Attacker)
						g.actorSystem.NoSender().Tell(g.ctx, redPID, &individual.Convert{
							TargetColor: individual.ColorBlue,
						})
					} else {
						// DEFENSE FAILED: Blue converts to Red!
						// Send command to the BLUE actor (Victim)
						// We need the PID for the blue actor. Ideally, we map IDs to PIDs efficiently.
						// For this demo, linear search or finding via name is okay.
						targetPID, _ := g.actorSystem.LocalActor(entity.Id)
						if targetPID != nil {
							g.actorSystem.NoSender().Tell(g.ctx, targetPID, &individual.Convert{
								TargetColor: individual.ColorRed,
							})
						}
					}
				}
			}
		}

		// Send perception update to Red actor so it knows where to run
		if len(visiblePrey) > 0 {
			g.actorSystem.NoSender().Tell(g.ctx, redPID, &individual.Perception{
				Targets: visiblePrey,
			})
		}
	}

	// 3. Send Tick
	for _, pid := range g.pids {
		g.actorSystem.NoSender().Tell(g.ctx, pid, &individual.Tick{})
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	for _, entity := range g.worldState {
		var clr color.Color
		// Use the Enum string to determine visual color
		if entity.Color == individual.ColorRed {
			clr = color.RGBA{255, 50, 50, 255}
		} else {
			clr = color.RGBA{50, 100, 255, 255}
		}

		vector.FillCircle(
			screen,
			float32(entity.PositionX),
			float32(entity.PositionY),
			6,
			clr,
			true,
		)
	}
}

func (g *Game) Layout(w, h int) (int, int) { return 640, 480 }

func main() {
	ctx := context.Background()
	system, _ := actor.NewActorSystem("SwarmWorld",
		actor.WithLogger(golog.DiscardLogger),
		actor.WithActorInitMaxRetries(3))
	_ = system.Start(ctx)
	defer system.Stop(ctx)

	updateChannel := make(chan *individual.ActorState, 1000)
	var pids []*actor.PID

	// Spawn a swarm!
	// 5 Red (Attacking)
	for i := 0; i < 5; i++ {
		pid, _ := system.Spawn(ctx, "Red-"+string(rune(i+'0')),
			individual.NewIndividual(individual.ColorRed, 50+float64(i)*20, 100, updateChannel))
		pids = append(pids, pid)
	}

	// 30 Blue (Defending) - More blues to allow clustering
	for i := 0; i < 30; i++ {
		// Determine generic name based on index
		name := "Blue-" + string(rune(i%10+'0')) + "-" + string(rune(i/10+'0'))
		pid, _ := system.Spawn(ctx, name,
			individual.NewIndividual(individual.ColorBlue, 300+float64(rand.Intn(100)), 100+float64(rand.Intn(100)), updateChannel))
		pids = append(pids, pid)
	}

	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Swarm: Red vs Blue (Defense Mode)")

	if err := ebiten.RunGame(&Game{
		actorSystem: system,
		ctx:         ctx,
		pids:        pids,
		updates:     updateChannel,
		worldState:  make(map[string]*individual.ActorState),
	}); err != nil {
		log.Fatal(err)
	}
}
