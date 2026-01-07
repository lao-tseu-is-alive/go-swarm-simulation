package sprite

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestNewPalette(t *testing.T) {
	p := NewPalette()
	if p.Len() != 0 {
		t.Errorf("expected empty palette, got %d colors", p.Len())
	}
}

func TestPalette_SetGetColor(t *testing.T) {
	p := NewPalette()
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}

	p.SetColor('R', red)

	got, ok := p.GetColor('R')
	if !ok {
		t.Fatal("expected color to be found")
	}
	if got != red {
		t.Errorf("expected %v, got %v", red, got)
	}

	// Test missing color
	_, ok = p.GetColor('X')
	if ok {
		t.Error("expected color not to be found")
	}
}

func TestPalette_Symbols(t *testing.T) {
	p := NewPalette()
	p.SetColor('G', color.RGBA{R: 0, G: 255, B: 0, A: 255})
	p.SetColor('R', color.RGBA{R: 255, G: 0, B: 0, A: 255})
	p.SetColor('B', color.RGBA{R: 0, G: 0, B: 255, A: 255})

	symbols := p.Symbols()
	if len(symbols) != 3 {
		t.Errorf("expected 3 symbols, got %d", len(symbols))
	}

	// Check insertion order
	expected := []rune{'G', 'R', 'B'}
	for i, s := range expected {
		if symbols[i] != s {
			t.Errorf("symbol %d: expected %c, got %c", i, s, symbols[i])
		}
	}
}

func TestPalette_RemoveColor(t *testing.T) {
	p := NewPalette()
	p.SetColor('R', color.RGBA{R: 255, G: 0, B: 0, A: 255})
	p.SetColor('G', color.RGBA{R: 0, G: 255, B: 0, A: 255})

	p.RemoveColor('R')

	if p.Len() != 1 {
		t.Errorf("expected 1 color, got %d", p.Len())
	}
	_, ok := p.GetColor('R')
	if ok {
		t.Error("expected color to be removed")
	}
}

func TestLoadSavePalette(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	palettePath := filepath.Join(tmpDir, "test.palette")

	// Create a palette
	p := NewPalette()
	p.SetColor('R', color.RGBA{R: 255, G: 0, B: 0, A: 255})
	p.SetColor('G', color.RGBA{R: 0, G: 255, B: 0, A: 128})
	p.SetColor('B', color.RGBA{R: 0, G: 0, B: 255, A: 255})

	// Save it
	err := SavePalette(palettePath, p)
	if err != nil {
		t.Fatalf("failed to save palette: %v", err)
	}

	// Load it back
	loaded, err := LoadPalette(palettePath)
	if err != nil {
		t.Fatalf("failed to load palette: %v", err)
	}

	// Verify
	if loaded.Len() != 3 {
		t.Errorf("expected 3 colors, got %d", loaded.Len())
	}

	r, ok := loaded.GetColor('R')
	if !ok {
		t.Fatal("expected R color")
	}
	if r.R != 255 || r.G != 0 || r.B != 0 || r.A != 255 {
		t.Errorf("unexpected R color: %v", r)
	}

	g, ok := loaded.GetColor('G')
	if !ok {
		t.Fatal("expected G color")
	}
	if g.A != 128 {
		t.Errorf("expected alpha 128, got %d", g.A)
	}
}

func TestLoadPalette_InvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content string
	}{
		{"missing values", "R 255"},
		{"invalid symbol", "RR 255 0 0"},
		{"invalid red", "R abc 0 0"},
		{"out of range", "R 256 0 0"},
		{"negative", "R -1 0 0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tc.name+".palette")
			err := os.WriteFile(path, []byte(tc.content), 0644)
			if err != nil {
				t.Fatal(err)
			}

			_, err = LoadPalette(path)
			if err == nil {
				t.Error("expected error for invalid format")
			}
		})
	}
}

func TestLoadPalette_CommentsAndEmptyLines(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "comments.palette")

	content := `# This is a comment
R 255 0 0 255

# Another comment
G 0 255 0 255
`
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	p, err := LoadPalette(path)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if p.Len() != 2 {
		t.Errorf("expected 2 colors, got %d", p.Len())
	}
}

func TestPalette_Clone(t *testing.T) {
	p := NewPalette()
	p.SetColor('R', color.RGBA{R: 255, G: 0, B: 0, A: 255})

	clone := p.Clone()

	// Modify original
	p.SetColor('R', color.RGBA{R: 100, G: 0, B: 0, A: 255})

	// Clone should be unchanged
	r, _ := clone.GetColor('R')
	if r.R != 255 {
		t.Errorf("clone was modified, expected R=255, got %d", r.R)
	}
}

func TestPalette_ToMap(t *testing.T) {
	p := NewPalette()
	p.SetColor('R', color.RGBA{R: 255, G: 0, B: 0, A: 255})
	p.SetColor('G', color.RGBA{R: 0, G: 255, B: 0, A: 255})

	m := p.ToMap()

	if len(m) != 2 {
		t.Errorf("expected 2 entries, got %d", len(m))
	}
	if m['R'].R != 255 {
		t.Error("unexpected R color in map")
	}
}

func TestLoadPalette_DefaultAlpha(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "noalpha.palette")

	// Color without alpha value
	content := "R 255 0 0\n"
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	p, err := LoadPalette(path)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	r, ok := p.GetColor('R')
	if !ok {
		t.Fatal("expected R color")
	}
	if r.A != 255 {
		t.Errorf("expected default alpha 255, got %d", r.A)
	}
}
