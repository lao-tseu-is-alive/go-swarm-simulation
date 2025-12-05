package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Checkbox is a simple UI widget for boolean values
type Checkbox struct {
	Label   string
	Value   bool
	X, Y    float64
	Size    float64
	clicked bool // Track if already clicked this frame
}

// NewCheckbox creates a new checkbox instance
func NewCheckbox(x, y float64, label string, value bool) *Checkbox {
	return &Checkbox{
		Label: label,
		Value: value,
		X:     x,
		Y:     y,
		Size:  16, // Default size
	}
}

// Update checks for mouse interaction
func (c *Checkbox) Update() {
	mx, my := ebiten.CursorPosition()

	// Check if mouse is over the checkbox
	isOver := float64(mx) >= c.X && float64(mx) <= c.X+c.Size &&
		float64(my) >= c.Y && float64(my) <= c.Y+c.Size

	// Toggle on click (with debouncing)
	if isOver && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if !c.clicked {
			c.Value = !c.Value
			c.clicked = true
		}
	} else {
		c.clicked = false
	}
}

// Draw renders the checkbox
func (c *Checkbox) Draw(screen *ebiten.Image) {
	// Draw box border
	vector.StrokeRect(screen,
		float32(c.X), float32(c.Y),
		float32(c.Size), float32(c.Size),
		2,
		color.RGBA{R: 200, G: 200, B: 200, A: 255},
		true)

	// Fill if checked
	if c.Value {
		vector.FillRect(screen,
			float32(c.X+2), float32(c.Y+2),
			float32(c.Size-4), float32(c.Size-4),
			color.RGBA{R: 100, G: 200, B: 100, A: 255},
			true)
	}
}
