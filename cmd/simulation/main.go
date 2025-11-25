package main

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/tochemey/goakt/v3/actor"

	// Use your specific import path here
	"github.com/lao-tseu-is-alive/go-swarm-simulation/internal/individual"
	golog "github.com/tochemey/goakt/v3/log"
)

func displayPosition(ctx context.Context, actorPID *actor.PID) error {
	res, err := actor.Ask(ctx, actorPID, &individual.GetState{}, 1*time.Second)
	if err != nil {
		fmt.Printf("Error asking actor %v: %v\n", actorPID, err)
		return err
	}
	// Cast the response to the expected type
	if state, ok := res.(*individual.ActorState); ok {
		fmt.Printf("%8s %12s is at [%.0f, %.0f]\n", state.Color, state.Id, state.PositionX, state.PositionY)
	}
	return nil
}

// Game implements ebiten.Game interface
type Game struct {
	actorSystem actor.ActorSystem
	ctx         context.Context

	// PIDs of our swarm
	pids []*actor.PID

	// Bridge: Channel to receive updates from actors
	updates chan *individual.ActorState

	// View: The current state of the world for rendering
	// Map ID -> State
	worldState map[string]*individual.ActorState
}

func (g *Game) Update() error {
	// 1. Drain the channel: Process all updates sent by actors since the last frame
Loop:
	for {
		select {
		case state := <-g.updates:
			// Update our local view of the world
			g.worldState[state.Id] = state
		default:
			// Channel is empty, stop reading
			break Loop
		}
	}

	// 2. Send Tick to all actors to make them move
	// Note: Ebiten calls Update() 60 times per second by default.
	for _, pid := range g.pids {
		// We use Tell (Fire-and-Forget) for maximum performance
		g.actorSystem.NoSender().Tell(g.ctx, pid, &individual.Tick{})
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Iterate over our world state and draw every individual
	for _, entity := range g.worldState {

		// Choose color
		var clr color.Color
		if entity.Color == "🔴 RED" {
			clr = color.RGBA{R: 255, G: 50, B: 50, A: 255}
		} else {
			clr = color.RGBA{R: 50, G: 50, B: 255, A: 255}
		}

		// Draw a circle
		vector.FillCircle(
			screen,
			float32(entity.PositionX),
			float32(entity.PositionY),
			5, // Radius
			clr,
			true, // Antialiasing
		)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 640, 480
}

func main() {
	ctx := context.Background()

	// 1. Init Actor System
	system, err := actor.NewActorSystem("SwarmWorld",
		actor.WithLogger(golog.DiscardLogger), // Reduce log noise in console
		actor.WithActorInitMaxRetries(3))
	if err != nil {
		panic(err)
	}
	if err := system.Start(ctx); err != nil {
		panic(err)
	}
	defer system.Stop(ctx)

	// 2. Create the channel bridge
	// Buffer it slightly to handle bursts
	updateChannel := make(chan *individual.ActorState, 1000)

	// 3. Spawn Swarm
	var pids []*actor.PID

	// Spawn 10 Red Aggressive ones
	for i := 0; i < 10; i++ {
		pid, _ := system.Spawn(ctx, "Red-"+string(rune(i+'0')),
			individual.NewIndividual("🔴 RED", 100, 100, updateChannel))
		pids = append(pids, pid)
	}

	// Spawn 10 Blue Calm ones
	for i := 0; i < 10; i++ {
		pid, _ := system.Spawn(ctx, "Blue-"+string(rune(i+'0')),
			individual.NewIndividual("🔵 BLUE", 400, 300, updateChannel))
		pids = append(pids, pid)
	}

	// 4. Start Game Loop
	game := &Game{
		actorSystem: system,
		ctx:         ctx,
		pids:        pids,
		updates:     updateChannel,
		worldState:  make(map[string]*individual.ActorState),
	}

	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("GoAkt Swarm Simulation")

	// Block until window is closed
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
