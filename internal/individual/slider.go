package individual

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Slider is a simple UI widget
type Slider struct {
	Label    string
	Value    float64
	Min, Max float64
	X, Y     float64
	W, H     float64
}

// Update checks for mouse interaction
func (s *Slider) Update() {
	mx, my := ebiten.CursorPosition()
	// Check if mouse is clicking inside the slider area
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if float64(mx) >= s.X && float64(mx) <= s.X+s.W &&
			float64(my) >= s.Y && float64(my) <= s.Y+s.H {
			// Calculate value based on horizontal position
			p := (float64(mx) - s.X) / s.W
			s.Value = s.Min + p*(s.Max-s.Min)

			// Clamp value
			if s.Value < s.Min {
				s.Value = s.Min
			}
			if s.Value > s.Max {
				s.Value = s.Max
			}
		}
	}
}

// Draw renders the slider
func (s *Slider) Draw(screen *ebiten.Image) {
	// Draw Background (Dark Gray)
	vector.FillRect(screen, float32(s.X), float32(s.Y), float32(s.W), float32(s.H), color.RGBA{R: 80, G: 80, B: 80, A: 255}, true)

	// Draw Value Bar (Light Gray/White)
	ratio := (s.Value - s.Min) / (s.Max - s.Min)
	vector.FillRect(screen, float32(s.X), float32(s.Y), float32(s.W*ratio), float32(s.H), color.RGBA{R: 200, G: 200, B: 200, A: 255}, true)
}
