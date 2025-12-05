package simulation

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pb"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/ui"
	"github.com/tochemey/goakt/v3/actor"
)

type Game struct {
	ctx        context.Context
	System     actor.ActorSystem
	worldPID   *actor.PID
	snapshotCh chan *pb.WorldSnapshot
	lastState  *pb.WorldSnapshot

	// UI Controls
	panel *ui.UIPanel

	// Widget references for easy access
	widgetDetectionRadius  *ui.Slider
	widgetDefenseRadius    *ui.Slider
	widgetContactRadius    *ui.Slider
	widgetVisualRange      *ui.Slider
	widgetProtectedRange   *ui.Slider
	widgetMaxSpeed         *ui.Slider
	widgetMinSpeed         *ui.Slider
	widgetAggression       *ui.Slider
	widgetCenteringFactor  *ui.Slider
	widgetAvoidFactor      *ui.Slider
	widgetMatchingFactor   *ui.Slider
	widgetTurnFactor       *ui.Slider
	widgetNumRed           *ui.Slider
	widgetNumBlue          *ui.Slider
	widgetDisplayDetection *ui.Checkbox
	widgetDisplayDefense   *ui.Checkbox

	cfg *Config

	// Timing instrumentation
	lastUpdateDuration time.Duration
	lastDrawDuration   time.Duration
	updateAvg          float64 // Rolling average in ms
	drawAvg            float64 // Rolling average in ms
}

func GetNewGame(ctx context.Context, cfg *Config, system actor.ActorSystem) *Game {
	// 1. Create Channels for communication
	snapshotCh := make(chan *pb.WorldSnapshot, 10) // Buffer to avoid blocking

	// 2. Spawn World Actor
	// We pass the channel to the World so it can push updates to us.
	// Note: NewWorldActor signature is (snapshotCh, cfg)
	worldActor := NewWorldActor(snapshotCh, cfg)
	worldPID, err := system.Spawn(ctx, "world", worldActor)
	if err != nil {
		panic(fmt.Sprintf("Failed to spawn world: %v", err))
	}

	// 3. Initialize UI Panel with all configuration widgets
	panel := ui.NewUIPanel(10, 10, 280, float64(cfg.WorldHeight)-20)

	// Add sections and widgets
	panel.AddSection("Interaction Radii")
	widgetDetectionRadius := panel.AddSlider("Detection Radius", 10, 300, cfg.DetectionRadius)
	widgetDefenseRadius := panel.AddSlider("Defense Radius", 10, 300, cfg.DefenseRadius)
	widgetContactRadius := panel.AddSlider("Contact Radius", 5, 50, cfg.ContactRadius)
	widgetVisualRange := panel.AddSlider("Visual Range", 10, 150, cfg.VisualRange)
	widgetProtectedRange := panel.AddSlider("Protected Range", 5, 50, cfg.ProtectedRange)
	panel.EndSection()

	panel.AddSection("Physics & Behavior")
	widgetMaxSpeed := panel.AddSlider("Max Speed", 1, 10, cfg.MaxSpeed)
	widgetMinSpeed := panel.AddSlider("Min Speed", 0.5, 8, cfg.MinSpeed)
	widgetAggression := panel.AddSlider("Aggression", 0.1, 2.0, cfg.Aggression)
	panel.EndSection()

	panel.AddSection("Boids Flocking")
	widgetCenteringFactor := panel.AddSlider("Centering Factor", 0.0001, 0.01, cfg.CenteringFactor)
	widgetAvoidFactor := panel.AddSlider("Avoid Factor", 0.001, 0.2, cfg.AvoidFactor)
	widgetMatchingFactor := panel.AddSlider("Matching Factor", 0.001, 0.2, cfg.MatchingFactor)
	widgetTurnFactor := panel.AddSlider("Turn Factor", 0.05, 1.0, cfg.TurnFactor)
	panel.EndSection()

	panel.AddSection("Population (Restart Required)")
	widgetNumRed := panel.AddSlider("Red Actors", 1, 300, float64(cfg.NumRedAtStart))
	widgetNumBlue := panel.AddSlider("Blue Actors", 1, 1000, float64(cfg.NumBlueAtStart))
	panel.EndSection()

	panel.AddSection("Visualization")
	widgetDisplayDetection := panel.AddCheckbox("Show Detection Circle", cfg.DisplayDetectionCircle)
	widgetDisplayDefense := panel.AddCheckbox("Show Defense Circle", cfg.DisplayDefenseCircle)
	panel.EndSection()

	return &Game{
		ctx:                    ctx,
		System:                 system,
		worldPID:               worldPID,
		snapshotCh:             snapshotCh,
		lastState:              &pb.WorldSnapshot{}, // Avoid nil pointer
		panel:                  panel,
		widgetDetectionRadius:  widgetDetectionRadius,
		widgetDefenseRadius:    widgetDefenseRadius,
		widgetContactRadius:    widgetContactRadius,
		widgetVisualRange:      widgetVisualRange,
		widgetProtectedRange:   widgetProtectedRange,
		widgetMaxSpeed:         widgetMaxSpeed,
		widgetMinSpeed:         widgetMinSpeed,
		widgetAggression:       widgetAggression,
		widgetCenteringFactor:  widgetCenteringFactor,
		widgetAvoidFactor:      widgetAvoidFactor,
		widgetMatchingFactor:   widgetMatchingFactor,
		widgetTurnFactor:       widgetTurnFactor,
		widgetNumRed:           widgetNumRed,
		widgetNumBlue:          widgetNumBlue,
		widgetDisplayDetection: widgetDisplayDetection,
		widgetDisplayDefense:   widgetDisplayDefense,
		cfg:                    cfg,
	}
}

func (g *Game) Update() error {
	start := time.Now()
	defer func() {
		g.lastUpdateDuration = time.Since(start)
		// Rolling average (exponential moving average)
		g.updateAvg = g.updateAvg*0.95 + float64(g.lastUpdateDuration.Microseconds())/1000.0*0.05
	}()

	// 1. Update UI Panel
	g.panel.Update()

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
		// Send all updated configuration values to the world
		actor.Tell(g.ctx, g.worldPID, &pb.UpdateConfig{
			DetectionRadius:        g.widgetDetectionRadius.Value,
			DefenseRadius:          g.widgetDefenseRadius.Value,
			ContactRadius:          g.widgetContactRadius.Value,
			VisualRange:            g.widgetVisualRange.Value,
			ProtectedRange:         g.widgetProtectedRange.Value,
			MaxSpeed:               g.widgetMaxSpeed.Value,
			MinSpeed:               g.widgetMinSpeed.Value,
			Aggression:             g.widgetAggression.Value,
			CenteringFactor:        g.widgetCenteringFactor.Value,
			AvoidFactor:            g.widgetAvoidFactor.Value,
			MatchingFactor:         g.widgetMatchingFactor.Value,
			TurnFactor:             g.widgetTurnFactor.Value,
			NumRedAtStart:          int32(g.widgetNumRed.Value),
			NumBlueAtStart:         int32(g.widgetNumBlue.Value),
			DisplayDetectionCircle: g.widgetDisplayDetection.Value,
			DisplayDefenseCircle:   g.widgetDisplayDefense.Value,
		})

		// Trigger Simulation Step
		actor.Tell(g.ctx, g.worldPID, &pb.Tick{})
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	start := time.Now()
	defer func() {
		g.lastDrawDuration = time.Since(start)
		g.drawAvg = g.drawAvg*0.95 + float64(g.lastDrawDuration.Microseconds())/1000.0*0.05
	}()

	// 1. Draw all actors from the last known snapshot
	if g.lastState != nil {
		// Pre-allocate batched vertices for Blue boids (3 vertices per boid)
		blueCount := int(g.lastState.BlueCount)
		boidVertices := make([]ebiten.Vertex, 0, blueCount*3)
		boidIndices := make([]uint16, 0, blueCount*3)

		for _, entity := range g.lastState.Actors {
			if entity.Color == pb.TeamColor_TEAM_RED {
				if g.widgetDisplayDetection.Value {
					clr := color.RGBA{R: 255, G: 50, B: 50, A: 255}
					vector.StrokeCircle(
						screen,
						float32(entity.Position.X),
						float32(entity.Position.Y),
						float32(g.widgetDetectionRadius.Value),
						1,
						clr,
						true,
					)
				}
				// Use pre-rendered circle sprite (batched by Ebiten automatically)
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(entity.Position.X-float64(circleRadius), entity.Position.Y-float64(circleRadius))
				screen.DrawImage(redCircleImg, op)
			} else {
				// Blue Boids - Collect vertices for batched draw
				angle := math.Atan2(entity.Velocity.Y, entity.Velocity.X)
				tipX := entity.Position.X + math.Cos(angle)*6
				tipY := entity.Position.Y + math.Sin(angle)*6
				rightX := entity.Position.X + math.Cos(angle+2.5)*5
				rightY := entity.Position.Y + math.Sin(angle+2.5)*5
				leftX := entity.Position.X + math.Cos(angle-2.5)*5
				leftY := entity.Position.Y + math.Sin(angle-2.5)*5

				baseIdx := uint16(len(boidVertices))
				boidVertices = append(boidVertices,
					ebiten.Vertex{DstX: float32(tipX), DstY: float32(tipY), SrcX: 1, SrcY: 1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
					ebiten.Vertex{DstX: float32(rightX), DstY: float32(rightY), SrcX: 1, SrcY: 1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
					ebiten.Vertex{DstX: float32(leftX), DstY: float32(leftY), SrcX: 1, SrcY: 1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
				)
				boidIndices = append(boidIndices, baseIdx, baseIdx+1, baseIdx+2)

				// Optional: Draw Defense Radius ring
				if g.widgetDisplayDefense.Value {
					clr := color.RGBA{R: 50, G: 100, B: 255, A: 50}
					vector.StrokeCircle(
						screen,
						float32(entity.Position.X),
						float32(entity.Position.Y),
						float32(g.widgetDefenseRadius.Value),
						1,
						clr,
						true,
					)
				}
			}
		}

		// SINGLE batched draw call for all Blue boids
		if len(boidVertices) > 0 {
			screen.DrawTriangles(boidVertices, boidIndices, whiteImage, &ebiten.DrawTrianglesOptions{})
		}
	}

	// 2. Draw UI Panel
	g.panel.Draw(screen)

	// 3. Draw the New Stats Bar
	g.drawStatsBar(screen)

	// 4. Draw Game Over Overlay
	if g.lastState.IsGameOver {
		// Simple centered text
		msg := fmt.Sprintf("GAME OVER\n%s is the WINNER !", g.lastState.Winner)
		// You can use basic printing or fancy vector text here
		ebitenutil.DebugPrintAt(screen, msg, int(g.cfg.WorldWidth/2-40), int(g.cfg.WorldHeight/2))
	}

	// Display timing breakdown for performance analysis
	// Display performance stats (moved to right side to avoid overlap with panel)
	msg := fmt.Sprintf("FPS: %.2f\nTPS: %.2f\n\nUpdate: %.2fms\nDraw:   %.2fms\nTotal:  %.2fms",
		ebiten.ActualFPS(),
		ebiten.ActualTPS(),
		g.updateAvg,
		g.drawAvg,
		g.updateAvg+g.drawAvg)
	// Print stats on the right side
	ebitenutil.DebugPrintAt(screen, msg, int(g.cfg.WorldWidth)-150, 10)

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

// Pre-rendered sprites for fast batched drawing
var (
	whiteImage   = ebiten.NewImage(3, 3)
	redCircleImg *ebiten.Image
	circleRadius = 6
)

func init() {
	whiteImage.Fill(color.RGBA{R: 100, G: 200, B: 255, A: 255})

	// Pre-render a red filled circle (much faster than vector.FillCircle each frame)
	size := circleRadius * 2
	redCircleImg = ebiten.NewImage(size, size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x - circleRadius)
			dy := float64(y - circleRadius)
			if dx*dx+dy*dy <= float64(circleRadius*circleRadius) {
				redCircleImg.Set(x, y, color.RGBA{R: 255, G: 50, B: 50, A: 255})
			}
		}
	}
}

func drawBoid(screen *ebiten.Image, b *pb.ActorState) {
	angle := math.Atan2(b.Velocity.Y, b.Velocity.X)

	// Visual geometry logic
	tipX := b.Position.X + math.Cos(angle)*6
	tipY := b.Position.Y + math.Sin(angle)*6
	rightX := b.Position.X + math.Cos(angle+2.5)*5
	rightY := b.Position.Y + math.Sin(angle+2.5)*5
	leftX := b.Position.X + math.Cos(angle-2.5)*5
	leftY := b.Position.Y + math.Sin(angle-2.5)*5

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
