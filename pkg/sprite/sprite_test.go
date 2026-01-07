package sprite

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSprite(t *testing.T) {
	s := NewSprite(10, 5)

	if s.Width() != 10 {
		t.Errorf("expected width 10, got %d", s.Width())
	}
	if s.Height() != 5 {
		t.Errorf("expected height 5, got %d", s.Height())
	}

	// Check all pixels are '.'
	for y := 0; y < 5; y++ {
		for x := 0; x < 10; x++ {
			if s.GetPixel(x, y) != '.' {
				t.Errorf("expected '.', got %c at (%d, %d)", s.GetPixel(x, y), x, y)
			}
		}
	}
}

func TestSprite_GetSetPixel(t *testing.T) {
	s := NewSprite(5, 5)

	s.SetPixel(2, 3, 'X')
	if s.GetPixel(2, 3) != 'X' {
		t.Errorf("expected 'X', got %c", s.GetPixel(2, 3))
	}

	// Out of bounds should return '.' and not panic
	if s.GetPixel(-1, 0) != '.' {
		t.Error("expected '.' for negative x")
	}
	if s.GetPixel(0, 100) != '.' {
		t.Error("expected '.' for out of bounds y")
	}

	// SetPixel out of bounds should not panic
	s.SetPixel(-1, 0, 'X') // Should do nothing
	s.SetPixel(0, 100, 'X')
}

func TestSprite_GetColor(t *testing.T) {
	s := NewSprite(5, 5)
	s.Palette.SetColor('R', color.RGBA{R: 255, G: 0, B: 0, A: 255})
	s.SetPixel(2, 2, 'R')

	c := s.GetColor(2, 2)
	if c.R != 255 {
		t.Errorf("expected red, got %v", c)
	}

	// Undefined symbol should return transparent
	s.SetPixel(3, 3, 'X')
	c = s.GetColor(3, 3)
	if c.A != 0 {
		t.Errorf("expected transparent, got %v", c)
	}
}

func TestLoadSaveSprite(t *testing.T) {
	tmpDir := t.TempDir()
	designPath := filepath.Join(tmpDir, "test.sprite")
	palettePath := filepath.Join(tmpDir, "test.palette")

	// Create a palette file
	paletteContent := `R 255 0 0 255
G 0 255 0 255
B 0 0 255 255
`
	err := os.WriteFile(palettePath, []byte(paletteContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create a sprite file
	spriteContent := `# Test sprite
.RG.
RGGR
.RG.
`
	err = os.WriteFile(designPath, []byte(spriteContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Load sprite
	s, err := LoadSprite(designPath, palettePath)
	if err != nil {
		t.Fatalf("failed to load sprite: %v", err)
	}

	if s.Width() != 4 {
		t.Errorf("expected width 4, got %d", s.Width())
	}
	if s.Height() != 3 {
		t.Errorf("expected height 3, got %d", s.Height())
	}

	// Check specific pixels
	if s.GetPixel(1, 0) != 'R' {
		t.Errorf("expected 'R' at (1,0), got %c", s.GetPixel(1, 0))
	}
	if s.GetPixel(2, 1) != 'G' {
		t.Errorf("expected 'G' at (2,1), got %c", s.GetPixel(2, 1))
	}

	// Save and reload
	savedPath := filepath.Join(tmpDir, "saved.sprite")
	err = SaveSprite(savedPath, s)
	if err != nil {
		t.Fatalf("failed to save sprite: %v", err)
	}

	// Verify saved content exists
	content, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if len(content) == 0 {
		t.Error("saved file is empty")
	}
}

func TestSprite_ToImage(t *testing.T) {
	s := NewSprite(3, 3)
	s.Palette.SetColor('R', color.RGBA{R: 255, G: 0, B: 0, A: 255})
	s.SetPixel(1, 1, 'R')

	img := s.ToImage()

	if img.Bounds().Dx() != 3 || img.Bounds().Dy() != 3 {
		t.Errorf("unexpected image size: %v", img.Bounds())
	}
}

func TestSprite_Clone(t *testing.T) {
	s := NewSprite(5, 5)
	s.Palette.SetColor('R', color.RGBA{R: 255, G: 0, B: 0, A: 255})
	s.SetPixel(2, 2, 'R')

	clone := s.Clone()

	// Modify original
	s.SetPixel(2, 2, 'X')
	s.Palette.SetColor('R', color.RGBA{R: 100, G: 0, B: 0, A: 255})

	// Clone should be unchanged
	if clone.GetPixel(2, 2) != 'R' {
		t.Errorf("clone design was modified")
	}
	r, _ := clone.Palette.GetColor('R')
	if r.R != 255 {
		t.Errorf("clone palette was modified")
	}
}

func TestSprite_Resize(t *testing.T) {
	s := NewSprite(3, 3)
	s.SetPixel(1, 1, 'X')

	// Grow
	s.Resize(5, 5)
	if s.Width() != 5 || s.Height() != 5 {
		t.Errorf("expected 5x5, got %dx%d", s.Width(), s.Height())
	}
	if s.GetPixel(1, 1) != 'X' {
		t.Error("pixel lost during resize")
	}
	if s.GetPixel(4, 4) != '.' {
		t.Error("new pixel should be '.'")
	}

	// Shrink
	s.Resize(2, 2)
	if s.Width() != 2 || s.Height() != 2 {
		t.Errorf("expected 2x2, got %dx%d", s.Width(), s.Height())
	}
}

func TestSprite_Clear(t *testing.T) {
	s := NewSprite(3, 3)
	s.SetPixel(1, 1, 'X')

	s.Clear('O')

	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			if s.GetPixel(x, y) != 'O' {
				t.Errorf("expected 'O' at (%d, %d), got %c", x, y, s.GetPixel(x, y))
			}
		}
	}
}

func TestLoadSprite_MissingPalette(t *testing.T) {
	tmpDir := t.TempDir()
	designPath := filepath.Join(tmpDir, "test.sprite")

	err := os.WriteFile(designPath, []byte("...\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = LoadSprite(designPath, filepath.Join(tmpDir, "nonexistent.palette"))
	if err == nil {
		t.Error("expected error for missing palette")
	}
}

func TestLoadSprite_MissingDesign(t *testing.T) {
	tmpDir := t.TempDir()
	palettePath := filepath.Join(tmpDir, "test.palette")

	err := os.WriteFile(palettePath, []byte("R 255 0 0\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = LoadSprite(filepath.Join(tmpDir, "nonexistent.sprite"), palettePath)
	if err == nil {
		t.Error("expected error for missing design")
	}
}

func TestSprite_UnevenRows(t *testing.T) {
	tmpDir := t.TempDir()
	designPath := filepath.Join(tmpDir, "uneven.sprite")
	palettePath := filepath.Join(tmpDir, "uneven.palette")

	// Create palette
	err := os.WriteFile(palettePath, []byte("X 255 0 0\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create sprite with uneven rows
	content := `XX
XXXX
X
`
	err = os.WriteFile(designPath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	s, err := LoadSprite(designPath, palettePath)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	// Width should be the max row width
	if s.Width() != 4 {
		t.Errorf("expected width 4, got %d", s.Width())
	}

	// All rows should be padded
	for i, row := range s.Design {
		if len(row) != 4 {
			t.Errorf("row %d not padded: len=%d", i, len(row))
		}
	}
}
