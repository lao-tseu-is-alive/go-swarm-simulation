// Package sprite provides types and utilities for loading, saving, and manipulating
// pixel art sprites and color palettes from simple text files.
package sprite

import (
	"bufio"
	"fmt"
	"image/color"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Palette maps single-character symbols to RGBA colors.
// It is used to define the color scheme for sprites.
type Palette struct {
	colors  map[rune]color.RGBA
	symbols []rune // Maintains insertion order for iteration
}

// NewPalette creates an empty palette.
func NewPalette() *Palette {
	return &Palette{
		colors:  make(map[rune]color.RGBA),
		symbols: make([]rune, 0),
	}
}

// LoadPalette reads a palette from a text file.
// Format: One color per line as "SYMBOL R G B A" or "SYMBOL R G B" (A defaults to 255).
// Lines starting with # are comments. Empty lines are skipped.
func LoadPalette(path string) (*Palette, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open palette file: %w", err)
	}
	defer file.Close()

	p := NewPalette()
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 4 || len(parts) > 5 {
			return nil, fmt.Errorf("line %d: expected 'SYMBOL R G B [A]', got %q", lineNum, line)
		}

		// Parse symbol (first character only)
		if len(parts[0]) != 1 {
			return nil, fmt.Errorf("line %d: symbol must be single character, got %q", lineNum, parts[0])
		}
		symbol := rune(parts[0][0])

		// Parse R, G, B
		r, err := strconv.Atoi(parts[1])
		if err != nil || r < 0 || r > 255 {
			return nil, fmt.Errorf("line %d: invalid red value %q", lineNum, parts[1])
		}
		g, err := strconv.Atoi(parts[2])
		if err != nil || g < 0 || g > 255 {
			return nil, fmt.Errorf("line %d: invalid green value %q", lineNum, parts[2])
		}
		b, err := strconv.Atoi(parts[3])
		if err != nil || b < 0 || b > 255 {
			return nil, fmt.Errorf("line %d: invalid blue value %q", lineNum, parts[3])
		}

		// Parse A (optional, defaults to 255)
		a := 255
		if len(parts) == 5 {
			a, err = strconv.Atoi(parts[4])
			if err != nil || a < 0 || a > 255 {
				return nil, fmt.Errorf("line %d: invalid alpha value %q", lineNum, parts[4])
			}
		}

		p.SetColor(symbol, color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading palette file: %w", err)
	}

	return p, nil
}

// SavePalette writes a palette to a text file.
func SavePalette(path string, p *Palette) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create palette file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write header comment
	fmt.Fprintln(writer, "# Palette file")
	fmt.Fprintln(writer, "# Format: SYMBOL R G B A")

	// Write colors in symbol order
	for _, symbol := range p.symbols {
		c := p.colors[symbol]
		fmt.Fprintf(writer, "%c %d %d %d %d\n", symbol, c.R, c.G, c.B, c.A)
	}

	return nil
}

// GetColor returns the color for a symbol, or false if not found.
func (p *Palette) GetColor(symbol rune) (color.RGBA, bool) {
	c, ok := p.colors[symbol]
	return c, ok
}

// SetColor sets or updates the color for a symbol.
func (p *Palette) SetColor(symbol rune, c color.RGBA) {
	if _, exists := p.colors[symbol]; !exists {
		p.symbols = append(p.symbols, symbol)
	}
	p.colors[symbol] = c
}

// RemoveColor removes a symbol from the palette.
func (p *Palette) RemoveColor(symbol rune) {
	delete(p.colors, symbol)
	// Remove from symbols slice
	for i, s := range p.symbols {
		if s == symbol {
			p.symbols = append(p.symbols[:i], p.symbols[i+1:]...)
			break
		}
	}
}

// Symbols returns all symbols in the palette in insertion order.
func (p *Palette) Symbols() []rune {
	result := make([]rune, len(p.symbols))
	copy(result, p.symbols)
	return result
}

// SymbolsSorted returns all symbols in the palette sorted alphabetically.
func (p *Palette) SymbolsSorted() []rune {
	result := make([]rune, len(p.symbols))
	copy(result, p.symbols)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

// Len returns the number of colors in the palette.
func (p *Palette) Len() int {
	return len(p.colors)
}

// ToMap returns a copy of the palette as a map (for compatibility with generateSprite).
func (p *Palette) ToMap() map[rune]color.RGBA {
	result := make(map[rune]color.RGBA, len(p.colors))
	for k, v := range p.colors {
		result[k] = v
	}
	return result
}

// Clone creates a deep copy of the palette.
func (p *Palette) Clone() *Palette {
	clone := NewPalette()
	for _, s := range p.symbols {
		clone.SetColor(s, p.colors[s])
	}
	return clone
}
