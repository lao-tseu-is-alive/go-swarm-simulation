package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tochemey/goakt/v3/actor"
	"github.com/tochemey/goakt/v3/log"

	"github.com/lao-tseu-is-alive/go-swarm-simulation/internal/individual" // Import your generated protobuf // Import your actor package
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

func main() {
	ctx := context.Background()

	// 1. Initialize Actor System
	system, err := actor.NewActorSystem("SwarmWorld",
		actor.WithLogger(log.DefaultLogger),
		actor.WithActorInitMaxRetries(3))
	if err != nil {
		panic(err)
	}

	if err := system.Start(ctx); err != nil {
		panic(err)
	}

	// 2. Spawn Individuals
	// Red One
	redPID, err := system.Spawn(ctx, "Aggressive-1",
		individual.NewIndividual("🔴 RED", 100, 100))
	if err != nil {
		fmt.Printf("Error creating red individual: %v\n", err)
	}

	// Blue One
	bluePID, err := system.Spawn(ctx, "Calm-1",
		individual.NewIndividual("🔵 BLUE", 400, 300))
	if err != nil {
		fmt.Printf("Error creating blue individual: %v\n", err)
	}

	// 3. Simulation Loop (The Game Loop)
	ticker := time.NewTicker(500 * time.Millisecond)
	go func() {
		for range ticker.C {
			// Tell them to move
			system.NoSender().Tell(ctx, redPID, &individual.Tick{})
			system.NoSender().Tell(ctx, bluePID, &individual.Tick{})

			// Ask where they are
			// Note: Ask is synchronous for this demo, in real game loop we handle this differently
			err := displayPosition(ctx, redPID)
			if err != nil {
				continue
			}
			err = displayPosition(ctx, bluePID)
			if err != nil {
				continue
			}

		}
	}()

	// 4. Wait for Ctrl+C
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	<-interrupt

	system.Stop(ctx)
}
