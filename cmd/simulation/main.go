package main

import (
	"context"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/internal/simulation"
)

const (
	numRedAtStart   = 5
	numBlueAtStart  = 30
	detectionRadius = 5
	defenseRadius   = 5
)

func main() {
	ctx := context.Background()

	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Swarm: Red vs Blue (Defense Mode)")

	game := simulation.GetNewGame(ctx, numRedAtStart, numBlueAtStart, detectionRadius, defenseRadius)
	defer game.System.Stop(ctx)
	err := ebiten.RunGame(game)
	if err != nil {
		log.Fatal(err)
	}
}
