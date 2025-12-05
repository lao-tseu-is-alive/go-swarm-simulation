package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Button is a clickable UI button
type Button struct {
	Label   string
	X, Y    float64
	Width   float64
	Height  float64
	clicked bool   // Track if already clicked this frame
	OnClick func() // Callback function

	// Styling
	BGColor    color.RGBA
	HoverColor color.RGBA
	TextColor  color.RGBA
}

// NewButton creates a new button instance
func NewButton(x, y, width, height float64, label string, onClick func()) *Button {
	return &Button{
		Label:      label,
		X:          x,
		Y:          y,
		Width:      width,
		Height:     height,
		OnClick:    onClick,
		BGColor:    color.RGBA{R: 80, G: 120, B: 180, A: 255},
		HoverColor: color.RGBA{R: 100, G: 150, B: 220, A: 255},
		TextColor:  color.RGBA{R: 255, G: 255, B: 255, A: 255},
	}
}

// Update checks for mouse interaction
func (b *Button) Update() {
	mx, my := ebiten.CursorPosition()

	// Check if mouse is over the button
	isOver := float64(mx) >= b.X && float64(mx) <= b.X+b.Width &&
		float64(my) >= b.Y && float64(my) <= b.Y+b.Height

	// Handle click
	if isOver && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if !b.clicked && b.OnClick != nil {
			b.OnClick()
			b.clicked = true
		}
	} else {
		b.clicked = false
	}
}

// Draw renders the button
func (b *Button) Draw(screen *ebiten.Image) {
	mx, my := ebiten.CursorPosition()
	isOver := float64(mx) >= b.X && float64(mx) <= b.X+b.Width &&
		float64(my) >= b.Y && float64(my) <= b.Y+b.Height

	// Choose color based on hover state
	bgColor := b.BGColor
	if isOver {
		bgColor = b.HoverColor
	}

	// Draw button background
	vector.FillRect(screen,
		float32(b.X), float32(b.Y),
		float32(b.Width), float32(b.Height),
		bgColor, true)

	// Draw border
	vector.StrokeRect(screen,
		float32(b.X), float32(b.Y),
		float32(b.Width), float32(b.Height),
		2, color.RGBA{R: 200, G: 200, B: 200, A: 255}, true)
}
