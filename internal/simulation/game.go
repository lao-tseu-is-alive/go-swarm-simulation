package simulation

import (
	"context"
	"fmt"
	"image/color"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/tochemey/goakt/v3/actor"
	golog "github.com/tochemey/goakt/v3/log"
)

type Game struct {
	System     actor.ActorSystem
	ctx        context.Context
	pids       []*actor.PID
	updates    chan *ActorState
	worldState map[string]*ActorState
	// UI Sliders
	sliderDetection *Slider
	sliderDefense   *Slider
}

func (g *Game) Update() error {
	// 1. Update Sliders (Check for Input)
	g.sliderDetection.Update()
	g.sliderDefense.Update()
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
	// Using Dynamic Slider Values
	// Get raw values from UI
	detRadius := g.sliderDetection.Value
	defRadius := g.sliderDefense.Value

	// Square them for distance comparison
	detectionDistSq := detRadius * detRadius // Large detection range for chasing
	defenseDistSq := defRadius * defRadius

	contactDistSq := 12.0 * 12.0 // Collision distance (radius 5 + 5 + margin)

	// Note: We iterate only through RED actors to drive the interactions
	// This is an O(N*M) operation. Fine for < 500 actors.
	for _, redPID := range g.pids {
		redState, ok := g.worldState[redPID.Name()]
		if !ok || redState.Color != ColorRed {
			continue
		}

		var visiblePrey []*ActorState

		// Scan for Blue targets
		for _, entity := range g.worldState {
			if entity.Color == ColorBlue {
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
						if defender.Color == ColorBlue && defender.Id != entity.Id {
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
						g.System.NoSender().Tell(g.ctx, redPID, &Convert{
							TargetColor: ColorBlue,
						})
					} else {
						// DEFENSE FAILED: Blue converts to Red!
						// Send command to the BLUE actor (Victim)
						// We need the PID for the blue actor. Ideally, we map IDs to PIDs efficiently.
						// For this demo, linear search or finding via name is okay.
						targetPID, _ := g.System.LocalActor(entity.Id)
						if targetPID != nil {
							g.System.NoSender().Tell(g.ctx, targetPID, &Convert{
								TargetColor: ColorRed,
							})
						}
					}
				}
			}
		}

		// Send perception update to Red actor so it knows where to run
		if len(visiblePrey) > 0 {
			g.System.NoSender().Tell(g.ctx, redPID, &Perception{
				Targets: visiblePrey,
			})
		}
	}

	// 3. Send Tick
	for _, pid := range g.pids {
		g.System.NoSender().Tell(g.ctx, pid, &Tick{})
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	for _, entity := range g.worldState {
		var clr color.Color
		// Use the Enum string to determine visual color
		if entity.Color == ColorRed {
			clr = color.RGBA{R: 255, G: 50, B: 50, A: 255}
		} else {
			clr = color.RGBA{R: 50, G: 100, B: 255, A: 255}
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
	// 2. Draw UI
	g.sliderDetection.Draw(screen)
	g.sliderDefense.Draw(screen)

	// 3. Draw Labels/Values
	// ebitenutil.DebugPrint draws at (0,0), so we use newlines to position text
	msg := fmt.Sprintf("Detection: %.0f\n\n\nDefense: %.0f",
		g.sliderDetection.Value,
		g.sliderDefense.Value)
	ebitenutil.DebugPrint(screen, msg)
}

func (g *Game) Layout(w, h int) (int, int) { return 640, 480 }

func GetNewGame(ctx context.Context, numRed, numBlue int, detectionRadius, defenseRadius float64) *Game {

	system, _ := actor.NewActorSystem("SwarmWorld",
		actor.WithLogger(golog.DiscardLogger),
		actor.WithActorInitMaxRetries(3))
	_ = system.Start(ctx)

	updateChannel := make(chan *ActorState, 1000)
	var pids []*actor.PID

	// Spawn a swarm of Red (Attacking)
	for i := 0; i < numRed; i++ {
		pid, _ := system.Spawn(ctx, "Red-"+string(rune(i+'0')),
			NewIndividual(ColorRed, 50+float64(i)*20, 100, updateChannel))
		pids = append(pids, pid)
	}

	// Spawn more Blue (Defending) to allow clustering
	for i := 0; i < numBlue; i++ {
		// Determine generic name based on index
		name := "Blue-" + string(rune(i%10+'0')) + "-" + string(rune(i/10+'0'))
		pid, _ := system.Spawn(ctx, name,
			NewIndividual(ColorBlue, 300+float64(rand.Intn(100)), 100+float64(rand.Intn(100)), updateChannel))
		pids = append(pids, pid)
	}

	// Initialize Sliders
	// Detection: Range 0 to 300, Start at 200, Positioned at top-left
	sDet := &Slider{
		Label: "Detection Radius",
		Value: detectionRadius,
		Min:   0, Max: 300,
		X: 10, Y: 20, W: 200, H: 20,
	}

	// Defense: Range 0 to 100, Start at 25, Positioned below Detection
	sDef := &Slider{
		Label: "Defense Radius",
		Value: defenseRadius,
		Min:   0, Max: 100,
		X: 10, Y: 70, W: 200, H: 20,
	}

	return &Game{
		System:          system,
		ctx:             ctx,
		pids:            pids,
		updates:         updateChannel,
		worldState:      make(map[string]*ActorState),
		sliderDetection: sDet,
		sliderDefense:   sDef,
	}
}
