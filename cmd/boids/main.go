package main

import (
	"image/color"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/behavior"
)

const (
	screenWidth  = 800
	screenHeight = 600
	numBoids     = 250
)

type Game struct {
	flock    []*behavior.Boid
	settings behavior.Settings
}

func (g *Game) Update() error {
	for _, b := range g.flock {
		// We pass the entire flock and the current settings to the boid
		b.Update(g.flock, g.settings)
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 10, G: 10, B: 30, A: 255})

	// Use the boid's exported X, Y, Vx, Vy to draw
	for _, b := range g.flock {
		drawBoid(screen, b)
	}
}

func drawBoid(screen *ebiten.Image, b *behavior.Boid) {
	angle := math.Atan2(b.Vy, b.Vx)

	// Visual geometry logic remains in main because it's specific to this view
	tipX := b.X + math.Cos(angle)*6
	tipY := b.Y + math.Sin(angle)*6
	rightX := b.X + math.Cos(angle+2.5)*5
	rightY := b.Y + math.Sin(angle+2.5)*5
	leftX := b.X + math.Cos(angle-2.5)*5
	leftY := b.Y + math.Sin(angle-2.5)*5

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

	// Fix 1: Removed FillRule.
	// For manual triangles, the default options are sufficient.
	op := &ebiten.DrawTrianglesOptions{}

	screen.DrawTriangles(vertices, indices, whiteImage, op)
}

func (g *Game) Layout(w, h int) (int, int) {
	return screenWidth, screenHeight
}

var whiteImage = ebiten.NewImage(3, 3)

func init() {
	whiteImage.Fill(color.RGBA{R: 100, G: 200, B: 255, A: 255})
}

func main() {
	// 1. Configure the specific rules for this simulation
	initialSettings := behavior.Settings{
		ScreenWidth:     screenWidth,
		ScreenHeight:    screenHeight,
		VisualRange:     70.0,
		ProtectedRange:  20.0,
		CenteringFactor: 0.0005,
		AvoidFactor:     0.05,
		MatchingFactor:  0.05,
		TurnFactor:      0.2,
		MaxSpeed:        4.0,
		MinSpeed:        2.0,
	}

	// 2. Initialize the flock
	flock := make([]*behavior.Boid, numBoids)
	for i := 0; i < numBoids; i++ {
		flock[i] = behavior.New(screenWidth, screenHeight)
	}

	g := &Game{
		flock:    flock,
		settings: initialSettings,
	}

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Boids (Cleaned)")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
