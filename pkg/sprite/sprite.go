// Package sprite provides types and utilities for loading, saving, and manipulating
// pixel art sprites and color palettes from simple text files.
package sprite

import (
	"bufio"
	"fmt"
	"image/color"
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// Sprite represents a pixel art sprite with an associated palette.
type Sprite struct {
	Design  []string // ASCII art rows
	Palette *Palette // Color definitions
}

// NewSprite creates a new empty sprite with the given dimensions.
func NewSprite(width, height int) *Sprite {
	design := make([]string, height)
	row := strings.Repeat(".", width)
	for i := range design {
		design[i] = row
	}
	return &Sprite{
		Design:  design,
		Palette: NewPalette(),
	}
}

// LoadSprite loads a sprite from separate design and palette files.
// The design file contains ASCII art (one row per line).
// Lines starting with # are comments. Empty lines are skipped.
func LoadSprite(designPath, palettePath string) (*Sprite, error) {
	// Load palette first
	palette, err := LoadPalette(palettePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load palette: %w", err)
	}

	// Load design
	file, err := os.Open(designPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sprite design file: %w", err)
	}
	defer file.Close()

	var design []string
	scanner := bufio.NewScanner(file)
	maxWidth := 0

	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments (but not empty lines, which could be part of the design)
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}

		design = append(design, line)
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading sprite design file: %w", err)
	}

	// Normalize row widths (pad shorter rows with spaces)
	for i, row := range design {
		if len(row) < maxWidth {
			design[i] = row + strings.Repeat(" ", maxWidth-len(row))
		}
	}

	return &Sprite{
		Design:  design,
		Palette: palette,
	}, nil
}

// SaveSprite writes the sprite design to a file.
// The palette is saved separately using SavePalette.
func SaveSprite(designPath string, s *Sprite) error {
	file, err := os.Create(designPath)
	if err != nil {
		return fmt.Errorf("failed to create sprite design file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write header comment
	fmt.Fprintln(writer, "# Sprite design file")

	// Write design rows
	for _, row := range s.Design {
		fmt.Fprintln(writer, row)
	}

	return nil
}

// ToImage generates an Ebiten image from the sprite design and palette.
func (s *Sprite) ToImage() *ebiten.Image {
	if len(s.Design) == 0 {
		return ebiten.NewImage(1, 1)
	}

	h := len(s.Design)
	w := len(s.Design[0])
	img := ebiten.NewImage(w, h)

	for y, row := range s.Design {
		for x, char := range row {
			if col, ok := s.Palette.GetColor(char); ok {
				img.Set(x, y, col)
			}
			// Transparent (no color set) for unknown symbols
		}
	}

	return img
}

// Width returns the width of the sprite in pixels.
func (s *Sprite) Width() int {
	if len(s.Design) == 0 {
		return 0
	}
	return len(s.Design[0])
}

// Height returns the height of the sprite in pixels.
func (s *Sprite) Height() int {
	return len(s.Design)
}

// GetPixel returns the symbol at the given position.
// Returns '.' if out of bounds.
func (s *Sprite) GetPixel(x, y int) rune {
	if y < 0 || y >= len(s.Design) {
		return '.'
	}
	row := s.Design[y]
	if x < 0 || x >= len(row) {
		return '.'
	}
	return rune(row[x])
}

// SetPixel sets the symbol at the given position.
// Does nothing if out of bounds.
func (s *Sprite) SetPixel(x, y int, symbol rune) {
	if y < 0 || y >= len(s.Design) {
		return
	}
	row := []rune(s.Design[y])
	if x < 0 || x >= len(row) {
		return
	}
	row[x] = symbol
	s.Design[y] = string(row)
}

// GetColor returns the color at the given pixel position.
// Returns transparent black if out of bounds or no color defined.
func (s *Sprite) GetColor(x, y int) color.RGBA {
	symbol := s.GetPixel(x, y)
	if c, ok := s.Palette.GetColor(symbol); ok {
		return c
	}
	return color.RGBA{}
}

// Clone creates a deep copy of the sprite.
func (s *Sprite) Clone() *Sprite {
	designCopy := make([]string, len(s.Design))
	copy(designCopy, s.Design)
	return &Sprite{
		Design:  designCopy,
		Palette: s.Palette.Clone(),
	}
}

// Resize changes the sprite dimensions, preserving existing content.
// New pixels are filled with '.'.
func (s *Sprite) Resize(newWidth, newHeight int) {
	newDesign := make([]string, newHeight)
	emptyRow := strings.Repeat(".", newWidth)

	for y := 0; y < newHeight; y++ {
		if y < len(s.Design) {
			oldRow := s.Design[y]
			if len(oldRow) >= newWidth {
				newDesign[y] = oldRow[:newWidth]
			} else {
				newDesign[y] = oldRow + strings.Repeat(".", newWidth-len(oldRow))
			}
		} else {
			newDesign[y] = emptyRow
		}
	}

	s.Design = newDesign
}

// Clear fills the entire sprite with the given symbol.
func (s *Sprite) Clear(symbol rune) {
	w := s.Width()
	row := strings.Repeat(string(symbol), w)
	for i := range s.Design {
		s.Design[i] = row
	}
}
