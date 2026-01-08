// Package main provides a standalone sprite and palette editor.
// This editor allows you to:
// - Load and edit sprites and palettes from text files
// - View sprites in magnified (pixel editing) and 1:1 (preview) modes
// - Edit palette colors with RGB sliders
// - Add new colors to the palette
// - Save changes back to files
//
// Usage:
//
//	go run ./cmd/sprite-editor -sprite assets/sprites/red_spaceship.sprite -palette assets/sprites/red_spaceship.palette
package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/sprite"
)

const (
	screenWidth  = 900
	screenHeight = 700
	pixelSize    = 24 // Magnified pixel size
	previewScale = 4  // 1:1 preview scale for visibility
)

// Editor is the main sprite editor application
type Editor struct {
	sprite       *sprite.Sprite
	spritePath   string
	palettePath  string
	selectedRune rune
	dirty        bool // Has unsaved changes

	// Palette editing
	editingColor    bool
	editColorRune   rune
	editR, editG    int
	editB, editA    int
	colorSliderDrag int // 0=none, 1=R, 2=G, 3=B, 4=A

	// Add new color mode
	addingColor    bool
	newColorSymbol rune

	// UI state
	statusMsg string
}

// NewEditor creates a new sprite editor
func NewEditor(spritePath, palettePath string) (*Editor, error) {
	var spr *sprite.Sprite
	var err error

	if spritePath != "" && palettePath != "" {
		spr, err = sprite.LoadSprite(spritePath, palettePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load sprite: %w", err)
		}
	} else {
		// Create a new empty sprite
		spr = sprite.NewSprite(16, 16)
		spr.Palette.SetColor('X', color.RGBA{R: 255, G: 0, B: 0, A: 255})
		spr.Palette.SetColor('O', color.RGBA{R: 0, G: 255, B: 0, A: 255})
		spr.Palette.SetColor('#', color.RGBA{R: 0, G: 0, B: 255, A: 255})
	}

	return &Editor{
		sprite:       spr,
		spritePath:   spritePath,
		palettePath:  palettePath,
		selectedRune: '.',
		statusMsg:    "Ready. S=Save  Left-click=Paint  Right-click=Edit Color  N=New Color",
	}, nil
}

func (e *Editor) Update() error {
	// Handle keyboard shortcuts
	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		e.save()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyN) && !e.editingColor && !e.addingColor {
		e.startAddColor()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if e.editingColor {
			e.editingColor = false
			e.statusMsg = "Color editor closed."
		} else if e.addingColor {
			e.addingColor = false
			e.statusMsg = "Add color cancelled."
		} else {
			os.Exit(0)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if e.editingColor {
			e.editingColor = false
			e.statusMsg = fmt.Sprintf("Color '%c' saved.", e.editColorRune)
		} else if e.addingColor {
			e.finishAddColor()
		}
	}

	// Handle typing new symbol when adding color
	if e.addingColor {
		for r := 'A'; r <= 'Z'; r++ {
			if inpututil.IsKeyJustPressed(ebiten.Key(int(ebiten.KeyA) + int(r-'A'))) {
				e.newColorSymbol = r
				e.statusMsg = fmt.Sprintf("New color symbol: '%c' - Press Enter to add, ESC to cancel", r)
			}
		}
		for r := '0'; r <= '9'; r++ {
			if inpututil.IsKeyJustPressed(ebiten.Key(int(ebiten.Key0) + int(r-'0'))) {
				e.newColorSymbol = r
				e.statusMsg = fmt.Sprintf("New color symbol: '%c' - Press Enter to add, ESC to cancel", r)
			}
		}
	}

	// Handle mouse input
	mx, my := ebiten.CursorPosition()

	// Sprite editing area (magnified view)
	spriteAreaX := 20
	spriteAreaY := 60
	spriteAreaW := e.sprite.Width() * pixelSize
	spriteAreaH := e.sprite.Height() * pixelSize

	// Palette area
	paletteX := 20
	paletteY := spriteAreaY + spriteAreaH + 40

	// Handle left-click: painting and palette selection
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && !e.editingColor && !e.addingColor {
		// Check if clicking on sprite area
		if mx >= spriteAreaX && mx < spriteAreaX+spriteAreaW &&
			my >= spriteAreaY && my < spriteAreaY+spriteAreaH {
			px := (mx - spriteAreaX) / pixelSize
			py := (my - spriteAreaY) / pixelSize
			if e.sprite.GetPixel(px, py) != e.selectedRune {
				e.sprite.SetPixel(px, py, e.selectedRune)
				e.dirty = true
			}
		}

		// Check if clicking on palette (left-click selects color)
		e.handlePaletteLeftClick(mx, my, paletteX, paletteY)
	}

	// Handle right-click: palette color editing (separate from left-click block!)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) && !e.editingColor && !e.addingColor {
		e.handlePaletteRightClick(mx, my, paletteX, paletteY)
	}

	// Handle color slider dragging
	if e.editingColor {
		e.handleColorSliders(mx, my)
	}

	// Release slider drag
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		e.colorSliderDrag = 0
	}

	return nil
}

// handlePaletteLeftClick handles left-click on palette swatches (color selection)
func (e *Editor) handlePaletteLeftClick(mx, my, paletteX, paletteY int) {
	swatchSize := 30
	swatchGap := 5

	// Check transparent/eraser button first (always at position 0)
	eraserX := paletteX
	if mx >= eraserX && mx < eraserX+swatchSize && my >= paletteY && my < paletteY+swatchSize {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			e.selectedRune = '.'
			e.statusMsg = "Selected: '.' (Transparent/Eraser)"
			return
		}
	}

	// Check palette colors (offset by 1 for eraser button)
	symbols := e.sprite.Palette.Symbols()
	for i, sym := range symbols {
		x := paletteX + (i+1)*(swatchSize+swatchGap)

		if mx >= x && mx < x+swatchSize && my >= paletteY && my < paletteY+swatchSize {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				e.selectedRune = sym
				e.statusMsg = fmt.Sprintf("Selected: '%c'", sym)
				return
			}
		}
	}
}

// handlePaletteRightClick handles right-click on palette swatches (color editing)
func (e *Editor) handlePaletteRightClick(mx, my, paletteX, paletteY int) {
	swatchSize := 30
	swatchGap := 5

	// Palette colors start after eraser button
	symbols := e.sprite.Palette.Symbols()
	for i, sym := range symbols {
		x := paletteX + (i+1)*(swatchSize+swatchGap)

		if mx >= x && mx < x+swatchSize && my >= paletteY && my < paletteY+swatchSize {
			e.startColorEdit(sym)
			return
		}
	}
}

func (e *Editor) startColorEdit(sym rune) {
	c, _ := e.sprite.Palette.GetColor(sym)
	e.editingColor = true
	e.editColorRune = sym
	e.editR = int(c.R)
	e.editG = int(c.G)
	e.editB = int(c.B)
	e.editA = int(c.A)
	e.statusMsg = fmt.Sprintf("Editing '%c'. Drag sliders. ENTER=Save, ESC=Cancel", sym)
}

func (e *Editor) startAddColor() {
	e.addingColor = true
	e.newColorSymbol = 'A'
	e.statusMsg = "Type a letter/number for new color symbol, then ENTER to add"
}

func (e *Editor) finishAddColor() {
	if e.newColorSymbol == 0 || e.newColorSymbol == '.' {
		e.statusMsg = "Invalid symbol. Use a letter or number."
		return
	}
	if _, exists := e.sprite.Palette.GetColor(e.newColorSymbol); exists {
		e.statusMsg = fmt.Sprintf("Symbol '%c' already exists!", e.newColorSymbol)
		return
	}
	// Add new color with random-ish default
	e.sprite.Palette.SetColor(e.newColorSymbol, color.RGBA{R: 128, G: 128, B: 128, A: 255})
	e.dirty = true
	e.addingColor = false
	e.selectedRune = e.newColorSymbol
	e.statusMsg = fmt.Sprintf("Added color '%c'. Right-click to edit its color.", e.newColorSymbol)
}

func (e *Editor) handleColorSliders(mx, my int) {
	sliderX := 500
	sliderY := 400
	sliderW := 200
	sliderH := 20
	gap := 30

	// Check if just pressed to start dragging
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		for i := 0; i < 4; i++ {
			y := sliderY + i*gap
			if mx >= sliderX && mx <= sliderX+sliderW && my >= y && my <= y+sliderH {
				e.colorSliderDrag = i + 1
			}
		}
	}

	// Handle dragging
	if e.colorSliderDrag > 0 && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		val := (mx - sliderX) * 255 / sliderW
		if val < 0 {
			val = 0
		}
		if val > 255 {
			val = 255
		}

		switch e.colorSliderDrag {
		case 1:
			e.editR = val
		case 2:
			e.editG = val
		case 3:
			e.editB = val
		case 4:
			e.editA = val
		}

		// Update palette color in real-time
		e.sprite.Palette.SetColor(e.editColorRune, color.RGBA{
			R: uint8(e.editR),
			G: uint8(e.editG),
			B: uint8(e.editB),
			A: uint8(e.editA),
		})
		e.dirty = true
	}
}

func (e *Editor) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 40, G: 40, B: 50, A: 255})

	// Title
	title := "Sprite Editor"
	if e.dirty {
		title += " *"
	}
	ebitenutil.DebugPrintAt(screen, title, 20, 10)
	ebitenutil.DebugPrintAt(screen, e.statusMsg, 20, 30)

	// Draw magnified sprite
	spriteAreaX := 20
	spriteAreaY := 60
	e.drawMagnifiedSprite(screen, spriteAreaX, spriteAreaY)

	// Draw 1:1 preview
	previewX := spriteAreaX + e.sprite.Width()*pixelSize + 40
	previewY := spriteAreaY
	e.drawPreview(screen, previewX, previewY)

	// Draw palette
	paletteX := 20
	paletteY := spriteAreaY + e.sprite.Height()*pixelSize + 40
	e.drawPalette(screen, paletteX, paletteY)

	// Draw color editor if active
	if e.editingColor {
		e.drawColorEditor(screen, 480, 380)
	}

	// Draw add color dialog if active
	if e.addingColor {
		e.drawAddColorDialog(screen, 480, 380)
	}

	// Instructions
	instructions := "S=Save  N=New Color  ESC=Exit/Cancel  ENTER=Confirm"
	ebitenutil.DebugPrintAt(screen, instructions, 20, screenHeight-25)
}

func (e *Editor) drawMagnifiedSprite(screen *ebiten.Image, x, y int) {
	ebitenutil.DebugPrintAt(screen, "Magnified View:", x, y-15)

	for py := 0; py < e.sprite.Height(); py++ {
		for px := 0; px < e.sprite.Width(); px++ {
			sym := e.sprite.GetPixel(px, py)

			// Draw checkerboard for transparent pixels
			if sym == '.' {
				e.drawCheckerboard(screen, x+px*pixelSize, y+py*pixelSize, pixelSize-1)
			} else {
				c := e.sprite.GetColor(px, py)
				vector.FillRect(screen,
					float32(x+px*pixelSize), float32(y+py*pixelSize),
					float32(pixelSize-1), float32(pixelSize-1),
					c, true)
			}

			// Draw grid
			vector.StrokeRect(screen,
				float32(x+px*pixelSize), float32(y+py*pixelSize),
				float32(pixelSize), float32(pixelSize),
				1, color.RGBA{R: 60, G: 60, B: 70, A: 255}, true)
		}
	}
}

func (e *Editor) drawPreview(screen *ebiten.Image, x, y int) {
	ebitenutil.DebugPrintAt(screen, "Preview:", x, y-15)

	// Background for transparency
	vector.FillRect(screen,
		float32(x), float32(y),
		float32(e.sprite.Width()*previewScale), float32(e.sprite.Height()*previewScale),
		color.RGBA{R: 100, G: 100, B: 100, A: 255}, true)

	// Draw sprite image
	img := e.sprite.ToImage()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(previewScale, previewScale)
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(img, op)
}

func (e *Editor) drawPalette(screen *ebiten.Image, x, y int) {
	ebitenutil.DebugPrintAt(screen, "Palette (Right-click to edit):", x, y-15)

	swatchSize := 30
	gap := 5

	// Draw transparent/eraser button first (checkerboard pattern)
	eraserX := x
	e.drawCheckerboard(screen, eraserX, y, swatchSize)

	// Highlight if eraser selected
	eraserBorder := color.RGBA{R: 100, G: 100, B: 100, A: 255}
	if e.selectedRune == '.' {
		eraserBorder = color.RGBA{R: 255, G: 255, B: 0, A: 255}
	}
	vector.StrokeRect(screen, float32(eraserX), float32(y), float32(swatchSize), float32(swatchSize), 2, eraserBorder, true)
	ebitenutil.DebugPrintAt(screen, ".", eraserX+12, y+swatchSize+5)

	// Draw palette colors (offset by 1 for eraser button)
	symbols := e.sprite.Palette.Symbols()
	for i, sym := range symbols {
		sx := x + (i+1)*(swatchSize+gap)
		c, _ := e.sprite.Palette.GetColor(sym)

		// Draw swatch
		vector.FillRect(screen, float32(sx), float32(y), float32(swatchSize), float32(swatchSize), c, true)

		// Highlight selected
		borderColor := color.RGBA{R: 100, G: 100, B: 100, A: 255}
		if sym == e.selectedRune {
			borderColor = color.RGBA{R: 255, G: 255, B: 0, A: 255}
		}
		vector.StrokeRect(screen, float32(sx), float32(y), float32(swatchSize), float32(swatchSize), 2, borderColor, true)

		// Draw symbol
		ebitenutil.DebugPrintAt(screen, string(sym), sx+10, y+swatchSize+5)
	}
}

// drawCheckerboard draws a checkerboard pattern to indicate transparency
func (e *Editor) drawCheckerboard(screen *ebiten.Image, x, y, size int) {
	checkSize := 6
	light := color.RGBA{R: 200, G: 200, B: 200, A: 255}
	dark := color.RGBA{R: 120, G: 120, B: 120, A: 255}

	for cy := 0; cy < size; cy += checkSize {
		for cx := 0; cx < size; cx += checkSize {
			c := light
			if (cx/checkSize+cy/checkSize)%2 == 0 {
				c = dark
			}
			w := checkSize
			h := checkSize
			if cx+w > size {
				w = size - cx
			}
			if cy+h > size {
				h = size - cy
			}
			vector.FillRect(screen, float32(x+cx), float32(y+cy), float32(w), float32(h), c, true)
		}
	}
}

func (e *Editor) drawColorEditor(screen *ebiten.Image, x, y int) {
	// Background panel
	vector.FillRect(screen, float32(x), float32(y), 280, 200, color.RGBA{R: 60, G: 60, B: 70, A: 250}, true)
	vector.StrokeRect(screen, float32(x), float32(y), 280, 200, 2, color.RGBA{R: 200, G: 200, B: 200, A: 255}, true)

	title := fmt.Sprintf("Edit Color: '%c'", e.editColorRune)
	ebitenutil.DebugPrintAt(screen, title, x+10, y+10)
	ebitenutil.DebugPrintAt(screen, "ENTER=Save  ESC=Cancel", x+10, y+25)

	sliderX := x + 40
	sliderY := y + 50
	sliderW := 200
	sliderH := 15
	gap := 30

	labels := []string{"R", "G", "B", "A"}
	values := []int{e.editR, e.editG, e.editB, e.editA}
	colors := []color.RGBA{
		{R: 255, G: 100, B: 100, A: 255},
		{R: 100, G: 255, B: 100, A: 255},
		{R: 100, G: 100, B: 255, A: 255},
		{R: 200, G: 200, B: 200, A: 255},
	}

	for i := 0; i < 4; i++ {
		sy := sliderY + i*gap

		// Label
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s:%3d", labels[i], values[i]), sliderX-35, sy)

		// Slider background
		vector.FillRect(screen, float32(sliderX), float32(sy), float32(sliderW), float32(sliderH),
			color.RGBA{R: 40, G: 40, B: 50, A: 255}, true)

		// Slider fill
		fillW := float32(values[i]) * float32(sliderW) / 255.0
		vector.FillRect(screen, float32(sliderX), float32(sy), fillW, float32(sliderH), colors[i], true)

		// Slider border
		vector.StrokeRect(screen, float32(sliderX), float32(sy), float32(sliderW), float32(sliderH), 1,
			color.RGBA{R: 150, G: 150, B: 150, A: 255}, true)
	}

	// Preview of edited color
	previewColor := color.RGBA{R: uint8(e.editR), G: uint8(e.editG), B: uint8(e.editB), A: uint8(e.editA)}
	vector.FillRect(screen, float32(x+200), float32(y+10), 60, 30, previewColor, true)
	vector.StrokeRect(screen, float32(x+200), float32(y+10), 60, 30, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255}, true)
}

func (e *Editor) drawAddColorDialog(screen *ebiten.Image, x, y int) {
	// Background panel
	vector.FillRect(screen, float32(x), float32(y), 280, 80, color.RGBA{R: 60, G: 60, B: 70, A: 250}, true)
	vector.StrokeRect(screen, float32(x), float32(y), 280, 80, 2, color.RGBA{R: 200, G: 200, B: 200, A: 255}, true)

	ebitenutil.DebugPrintAt(screen, "Add New Color", x+10, y+10)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Symbol: %c", e.newColorSymbol), x+10, y+30)
	ebitenutil.DebugPrintAt(screen, "Type A-Z or 0-9, then ENTER", x+10, y+50)
}

func (e *Editor) save() {
	if e.spritePath == "" || e.palettePath == "" {
		e.statusMsg = "Error: No file paths specified!"
		return
	}

	err := sprite.SaveSprite(e.spritePath, e.sprite)
	if err != nil {
		e.statusMsg = fmt.Sprintf("Error saving sprite: %v", err)
		return
	}

	err = sprite.SavePalette(e.palettePath, e.sprite.Palette)
	if err != nil {
		e.statusMsg = fmt.Sprintf("Error saving palette: %v", err)
		return
	}

	e.dirty = false
	e.statusMsg = "Saved successfully!"
}

func (e *Editor) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	spritePath := flag.String("sprite", "", "Path to sprite design file")
	palettePath := flag.String("palette", "", "Path to palette file")
	flag.Parse()

	// If no args, use defaults
	if *spritePath == "" {
		*spritePath = "assets/sprites/red_spaceship.sprite"
	}
	if *palettePath == "" {
		*palettePath = "assets/sprites/red_spaceship.palette"
	}

	editor, err := NewEditor(*spritePath, *palettePath)
	if err != nil {
		log.Fatalf("Failed to create editor: %v", err)
	}

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Sprite Editor")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	if err := ebiten.RunGame(editor); err != nil {
		log.Fatal(err)
	}
}
