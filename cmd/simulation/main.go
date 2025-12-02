package main

import (
	"context"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/simulation"
)

func main() {
	ctx := context.Background()
	// Load Config
	cfg, err := simulation.LoadConfig("config.json", "config_schema.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ebiten.SetWindowSize(int(cfg.WorldWidth), int(cfg.WorldHeight))
	ebiten.SetWindowTitle("Swarm: Red vs Blue (Defense Mode)")

	game := simulation.GetNewGame(ctx, cfg)
	defer game.System.Stop(ctx)
	err = ebiten.RunGame(game)
	if err != nil {
		log.Fatal(err)
	}
}
