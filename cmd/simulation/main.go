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
	defenseRadius   = 40
	worldWidth      = 1000
	worldHeight     = 800
)

func main() {
	ctx := context.Background()

	ebiten.SetWindowSize(worldWidth, worldHeight)
	ebiten.SetWindowTitle("Swarm: Red vs Blue (Defense Mode)")

	game := simulation.GetNewGame(ctx, numRedAtStart, numBlueAtStart, detectionRadius, defenseRadius, worldWidth, worldHeight)
	defer game.System.Stop(ctx)
	err := ebiten.RunGame(game)
	if err != nil {
		log.Fatal(err)
	}
}
