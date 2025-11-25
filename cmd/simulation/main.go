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
	redPID, _ := system.Spawn(ctx, "Aggressive-1",
		individual.NewIndividual("RED", 100, 100))

	// Blue One
	bluePID, _ := system.Spawn(ctx, "Calm-1",
		individual.NewIndividual("BLUE", 400, 300))

	// 3. Simulation Loop (The Game Loop)
	ticker := time.NewTicker(500 * time.Millisecond)
	go func() {
		for range ticker.C {
			// Tell them to move
			system.NoSender().SendAsync(ctx, redPID.ID(), &individual.Tick{})
			system.NoSender().SendAsync(ctx, bluePID.ID(), &individual.Tick{})

			// Ask where they are
			// Note: Ask is synchronous for this demo, in real game loop we handle this differently
			res, _ := actor.Ask(ctx, redPID, &individual.GetState{}, 1*time.Second)
			state := res.(*individual.ActorState)
			fmt.Printf("🔴 RED is at [%.0f, %.0f]\n", state.PositionX, state.PositionY)
		}
	}()

	// 4. Wait for Ctrl+C
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	<-interrupt

	system.Stop(ctx)
}
