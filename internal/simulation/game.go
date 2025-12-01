package simulation

import (
	"context"
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/tochemey/goakt/v3/actor"
	golog "github.com/tochemey/goakt/v3/log"
)

type Game struct {
	System actor.ActorSystem
	ctx    context.Context

	// The World Actor Reference
	worldPID *actor.PID

	// Data from World
	snapshotCh chan *WorldSnapshot
	lastState  *WorldSnapshot

	// UI Sliders
	sliderDetection *Slider
	sliderDefense   *Slider

	cfg *Config
}

func GetNewGame(ctx context.Context, cfg *Config) *Game {
	// 1. Start Actor System
	system, _ := actor.NewActorSystem("SwarmWorld",
		actor.WithLogger(golog.DefaultLogger),
		actor.WithActorInitMaxRetries(3))
	_ = system.Start(ctx)

	// 2. Create the channel for World -> UI communication
	snapshotCh := make(chan *WorldSnapshot, 10)

	// 3. SPAWN THE WORLD ACTOR
	// Note: We pass the settings so the World can spawn the individuals
	worldActor := NewWorldActor(snapshotCh, cfg)
	worldPID, _ := system.Spawn(ctx, "World", worldActor)

	// 4. Initialize Sliders (UI only)
	sDet := &Slider{
		Label: "Detection", Value: cfg.DetectionRadius,
		Min: 0, Max: 300, X: 10, Y: 20, W: 200, H: 20,
	}
	sDef := &Slider{
		Label: "Defense", Value: cfg.DefenseRadius,
		Min: 0, Max: 100, X: 10, Y: 70, W: 200, H: 20,
	}

	return &Game{
		System:          system,
		ctx:             ctx,
		worldPID:        worldPID,
		snapshotCh:      snapshotCh,
		lastState:       &WorldSnapshot{}, // Avoid nil pointer
		sliderDetection: sDet,
		sliderDefense:   sDef,
		cfg:             cfg,
	}
}

func (g *Game) Update() error {
	// 1. Update UI Inputs
	g.sliderDetection.Update()
	g.sliderDefense.Update()

	// 2. Send Updated Config to World (Fire and Forget)
	// Only send if changed (optimization omitted for brevity)
	g.System.NoSender().Tell(g.ctx, g.worldPID, &UpdateConfig{
		DetectionRadius: g.sliderDetection.Value,
		DefenseRadius:   g.sliderDefense.Value,
	})

	// 3. Trigger Simulation Step
	// We tell the World: "It's time to process a frame"
	g.System.NoSender().Tell(g.ctx, g.worldPID, &Tick{})

	// 4. Retrieve Latest State (Non-blocking)
	select {
	case snap := <-g.snapshotCh:
		g.lastState = snap
	default:
		// Use previous state if new one isn't ready
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// 1. Draw all actors from the last known snapshot
	if g.lastState != nil {
		for _, entity := range g.lastState.Actors {
			var clr color.Color
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
	}

	// 2. Draw UI
	g.sliderDetection.Draw(screen)
	g.sliderDefense.Draw(screen)

	msg := fmt.Sprintf("Detection: %.0f\n\n\nDefense: %.0f",
		g.sliderDetection.Value,
		g.sliderDefense.Value)
	ebitenutil.DebugPrint(screen, msg)
}

func (g *Game) Layout(w, h int) (int, int) { return int(g.cfg.WorldWidth), int(g.cfg.WorldHeight) }
