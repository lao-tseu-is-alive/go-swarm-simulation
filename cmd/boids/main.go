package main

import (
	"image/color"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	// Import the local package
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

	path := vector.Path{}
	path.MoveTo(float32(tipX), float32(tipY))
	path.LineTo(float32(rightX), float32(rightY))
	path.LineTo(float32(leftX), float32(leftY))
	path.Close()

	vertices, indices := path.AppendVerticesAndIndicesForFilling(nil, nil)

	// Create a single color texture for the triangle
	whiteSubImage := whiteImage.SubImage(whiteImage.Bounds()).(*ebiten.Image)

	op := &ebiten.DrawTrianglesOptions{FillRule: ebiten.EvenOdd}
	screen.DrawTriangles(vertices, indices, whiteSubImage, op)
}

func (g *Game) Layout(w, h int) (int, int) {
	return screenWidth, screenHeight
}

// Shared texture resource
var whiteImage = ebiten.NewImage(3, 3)

func init() {
	whiteImage.Fill(color.RGBA{R: 100, G: 200, B: 255, A: 255})
	rand.Seed(time.Now().UnixNano())
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
	ebiten.SetWindowTitle("Refactored Boids")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
