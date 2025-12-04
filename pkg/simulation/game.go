package simulation

import (
	"context"
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/tochemey/goakt/v3/actor"
	golog "github.com/tochemey/goakt/v3/log"
	"google.golang.org/protobuf/runtime/protoimpl"
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
	sDetection := &Slider{
		Label: "Detection", Value: cfg.DetectionRadius,
		Min: 0, Max: 100, X: 10, Y: 20, W: 200, H: 20,
	}
	sDefense := &Slider{
		Label: "Defense", Value: cfg.DefenseRadius,
		Min: 0, Max: 100, X: 10, Y: 70, W: 200, H: 20,
	}

	return &Game{
		System:     system,
		ctx:        ctx,
		worldPID:   worldPID,
		snapshotCh: snapshotCh,
		lastState: &WorldSnapshot{
			state:         protoimpl.MessageState{},
			Actors:        nil,
			RedCount:      0,
			BlueCount:     0,
			IsGameOver:    false,
			Winner:        "",
			unknownFields: nil,
			sizeCache:     0,
		}, // Avoid nil pointer
		sliderDetection: sDetection,
		sliderDefense:   sDefense,
		cfg:             cfg,
	}
}

func (g *Game) Update() error {
	// 1. Update UI Inputs
	g.sliderDetection.Update()
	g.sliderDefense.Update()

	// 2. Retrieve Latest State (Non-blocking) EARLY, so we can check IsGameOver before ticking
	select {
	case snap := <-g.snapshotCh:
		g.lastState = snap
	default:
		// Use previous state if new one isn't ready
	}
	// ONLY send a Tick if the game is NOT over.
	// This effectively "freezes" the simulation in the final state.
	if !g.lastState.IsGameOver {
		// 3. Send Updated Config to World (Fire and Forget)
		// Only send if changed (optimization omitted for brevity)
		g.System.NoSender().Tell(g.ctx, g.worldPID, &UpdateConfig{
			DetectionRadius: g.sliderDetection.Value,
			DefenseRadius:   g.sliderDefense.Value,
		})

		// 4. Trigger Simulation Step
		// We tell the World: "It's time to process a frame"
		g.System.NoSender().Tell(g.ctx, g.worldPID, &Tick{})
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// 1. Draw all actors from the last known snapshot
	// 1. Draw all actors from the last known snapshot
	if g.lastState != nil {
		for _, entity := range g.lastState.Actors {
			var clr color.Color
			if entity.Color == TeamColor_TEAM_RED {
				clr = color.RGBA{R: 255, G: 50, B: 50, A: 255}
				vector.StrokeCircle(
					screen,
					float32(entity.PositionX),
					float32(entity.PositionY),
					float32(g.sliderDetection.Value),
					1,
					clr,
					true,
				)
				vector.FillCircle(
					screen,
					float32(entity.PositionX),
					float32(entity.PositionY),
					6,
					clr,
					true,
				)
			} else {
				// Blue Boids - Draw as Triangles
				drawBoid(screen, entity)

				// Optional: Draw Defense Radius ring if you want to see it
				clr = color.RGBA{R: 50, G: 100, B: 255, A: 50} // Transparent blue
				vector.StrokeCircle(
					screen,
					float32(entity.PositionX),
					float32(entity.PositionY),
					float32(g.sliderDefense.Value),
					1,
					clr,
					true,
				)
			}
		}
	}

	// 2. Draw UI
	g.sliderDetection.Draw(screen)
	g.sliderDefense.Draw(screen)

	// 3. Draw the New Stats Bar
	g.drawStatsBar(screen)

	// 4. Draw Game Over Overlay
	if g.lastState.IsGameOver {
		// Simple centered text
		msg := fmt.Sprintf("GAME OVER\n%s is the WINNER !", g.lastState.Winner)
		// You can use basic printing or fancy vector text here
		ebitenutil.DebugPrintAt(screen, msg, int(g.cfg.WorldWidth/2-40), int(g.cfg.WorldHeight/2))
	}

	msg := fmt.Sprintf("Detection: %.0f\n\n\nDefense: %.0f\n\n",
		g.sliderDetection.Value,
		g.sliderDefense.Value)
	ebitenutil.DebugPrint(screen, msg)

}

func (g *Game) drawStatsBar(screen *ebiten.Image) {
	if g.lastState == nil {
		return
	}

	reds := float32(g.lastState.RedCount)
	blues := float32(g.lastState.BlueCount)
	total := reds + blues

	// Avoid divide by zero at start
	if total == 0 {
		return
	}

	// --- Configuration ---
	barWidth := float32(200.0)
	barHeight := float32(20.0)
	marginTop := float32(10.0)
	marginRight := float32(10.0)

	// Calculate Position (Top Right)
	// screen.Bounds().Dx() gives current window width
	screenW := float32(screen.Bounds().Dx())
	x := screenW - barWidth - marginRight
	y := marginTop

	// Calculate Ratios
	redRatio := reds / total
	redW := barWidth * redRatio
	blueW := barWidth - redW

	// --- Draw Bars ---
	// 1. Red Bar (Left side of the stack)
	vector.FillRect(screen, x, y, redW, barHeight, color.RGBA{R: 255, G: 50, B: 50, A: 255}, true)

	// 2. Blue Bar (Right side, starts where Red ends)
	vector.FillRect(screen, x+redW, y, blueW, barHeight, color.RGBA{R: 50, G: 100, B: 255, A: 255}, true)

	// --- Draw Text Below ---
	// Position text under the respective colors

	// Red Count
	redMsg := fmt.Sprintf("%d", int(reds))
	ebitenutil.DebugPrintAt(screen, redMsg, int(x), int(y+barHeight+5))

	// Blue Count (Aligned to the end of the bar roughly)
	blueMsg := fmt.Sprintf("%d", int(blues))
	// A simple hack to align right: subtract estimated text width (approx 8px per char)
	textOffset := float32(len(blueMsg) * 8)
	ebitenutil.DebugPrintAt(screen, blueMsg, int(x+barWidth-textOffset), int(y+barHeight+5))
}

func (g *Game) Layout(w, h int) (int, int) { return int(g.cfg.WorldWidth), int(g.cfg.WorldHeight) }

var whiteImage = ebiten.NewImage(3, 3)

func init() {
	whiteImage.Fill(color.RGBA{R: 100, G: 200, B: 255, A: 255})
}

func drawBoid(screen *ebiten.Image, b *ActorState) {
	angle := math.Atan2(b.VelocityY, b.VelocityX)

	// Visual geometry logic
	tipX := b.PositionX + math.Cos(angle)*6
	tipY := b.PositionY + math.Sin(angle)*6
	rightX := b.PositionX + math.Cos(angle+2.5)*5
	rightY := b.PositionY + math.Sin(angle+2.5)*5
	leftX := b.PositionX + math.Cos(angle-2.5)*5
	leftY := b.PositionY + math.Sin(angle-2.5)*5

	// Define the 3 vertices of the triangle
	vertices := []ebiten.Vertex{
		{
			DstX: float32(tipX),
			DstY: float32(tipY),
			SrcX: 1, SrcY: 1,
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
		},
		{
			DstX: float32(rightX),
			DstY: float32(rightY),
			SrcX: 1, SrcY: 1,
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
		},
		{
			DstX: float32(leftX),
			DstY: float32(leftY),
			SrcX: 1, SrcY: 1,
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
		},
	}

	indices := []uint16{0, 1, 2}

	op := &ebiten.DrawTrianglesOptions{}

	screen.DrawTriangles(vertices, indices, whiteImage, op)
}
