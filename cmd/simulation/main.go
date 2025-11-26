package main

import (
	"context"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/internal/individual"
)

const numRedAtStart = 3
const numBlueAtStart = 20

func main() {
	ctx := context.Background()

	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Swarm: Red vs Blue (Defense Mode)")

	game := individual.GetNewGame(ctx, numRedAtStart, numBlueAtStart)
	defer game.System.Stop(ctx)
	err := ebiten.RunGame(game)
	if err != nil {
		log.Fatal(err)
	}
}
