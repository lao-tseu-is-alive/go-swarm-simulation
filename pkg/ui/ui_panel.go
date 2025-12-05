package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// UIWidget is an interface for all UI widgets
type UIWidget interface {
	Update()
	Draw(screen *ebiten.Image)
	GetHeight() float64
}

// SliderWrapper wraps our existing Slider to implement UIWidget
type SliderWrapper struct {
	*Slider
}

func (s *SliderWrapper) GetHeight() float64 {
	return s.H + 25 // Slider height + label space
}

// CheckboxWrapper wraps Checkbox to implement UIWidget
type CheckboxWrapper struct {
	*Checkbox
}

func (c *CheckboxWrapper) GetHeight() float64 {
	return c.Size + 25 // Checkbox size + label space + margin
}

// ButtonWrapper wraps Button to implement UIWidget
type ButtonWrapper struct {
	*Button
}

func (b *ButtonWrapper) GetHeight() float64 {
	return b.Height + 10 // Button height + margin
}

// UIPanel manages a collection of UI widgets in a scrollable panel
type UIPanel struct {
	X, Y          float64 // Panel position
	Width, Height float64 // Panel dimensions
	Widgets       []UIWidget
	Labels        []string // Labels for widgets
	ScrollOffset  float64  // Current scroll position

	// Styling
	BGColor     color.RGBA
	BorderColor color.RGBA
	TextColor   color.RGBA

	// Section headers
	sections []PanelSection

	// Slide animation
	TargetX     float64 // Target X position for animation
	slideSpeed  float64 // Pixels per frame
	IsCollapsed bool    // Whether panel is hidden

	// Hide button
	hideButton *Button
}

// PanelSection represents a collapsible section in the panel
type PanelSection struct {
	Title      string
	StartIndex int // Widget index where this section starts
	EndIndex   int // Widget index where this section ends (exclusive)
	Collapsed  bool
}

// NewUIPanel creates a new UI panel
func NewUIPanel(x, y, width, height float64) *UIPanel {
	panel := &UIPanel{
		X:            x,
		Y:            y,
		Width:        width,
		Height:       height,
		Widgets:      make([]UIWidget, 0),
		Labels:       make([]string, 0),
		ScrollOffset: 0,
		BGColor:      color.RGBA{R: 40, G: 40, B: 45, A: 230},
		BorderColor:  color.RGBA{R: 100, G: 100, B: 110, A: 255},
		TextColor:    color.RGBA{R: 220, G: 220, B: 220, A: 255},
		sections:     make([]PanelSection, 0),
		TargetX:      x,
		slideSpeed:   20.0,
		IsCollapsed:  false,
	}

	// Create hide button (top-right corner of panel)
	panel.hideButton = NewButton(
		x+width-30, // Right side
		y+5,        // Top
		25, 20,     // Small button
		"<", // Left arrow (ASCII)
		func() {
			panel.Toggle()
		},
	)

	return panel
}

// AddSection adds a section header
func (p *UIPanel) AddSection(title string) {
	p.sections = append(p.sections, PanelSection{
		Title:      title,
		StartIndex: len(p.Widgets),
		Collapsed:  false,
	})
}

// EndSection closes the current section
func (p *UIPanel) EndSection() {
	if len(p.sections) > 0 {
		p.sections[len(p.sections)-1].EndIndex = len(p.Widgets)
	}
}

// AddSlider adds a slider widget to the panel
func (p *UIPanel) AddSlider(label string, min, max, value float64) *Slider {
	// Calculate position within panel
	yOffset := p.calculateNextYOffset()

	slider := NewSlider(
		p.X+10,         // X position with margin
		p.Y+yOffset+20, // Y position
		p.Width-20,     // Width with margins
		label,
		min, max, value,
	)

	p.Widgets = append(p.Widgets, &SliderWrapper{slider})
	p.Labels = append(p.Labels, label)

	return slider
}

// AddCheckbox adds a checkbox widget to the panel
func (p *UIPanel) AddCheckbox(label string, value bool) *Checkbox {
	yOffset := p.calculateNextYOffset()

	checkbox := NewCheckbox(
		p.X+10,
		p.Y+yOffset+20,
		label,
		value,
	)

	p.Widgets = append(p.Widgets, &CheckboxWrapper{checkbox})
	p.Labels = append(p.Labels, label)

	return checkbox
}

// AddButton adds a button widget to the panel
func (p *UIPanel) AddButton(label string, onClick func()) *Button {
	yOffset := p.calculateNextYOffset()

	button := NewButton(
		p.X+10,
		p.Y+yOffset+20,
		p.Width-20,
		30,
		label,
		onClick,
	)

	p.Widgets = append(p.Widgets, &ButtonWrapper{button})
	p.Labels = append(p.Labels, label)

	return button
}

// calculateNextYOffset calculates the Y offset for the next widget
func (p *UIPanel) calculateNextYOffset() float64 {
	offset := 0.0

	// Add section header heights (20px each)
	for range p.sections {
		offset += 25
	}

	// Add all widget heights
	for _, widget := range p.Widgets {
		offset += widget.GetHeight()
	}

	return offset
}

// Update handles input for all widgets
func (p *UIPanel) Update() {
	// Handle slide animation
	if p.X != p.TargetX {
		diff := p.TargetX - p.X
		if diff > 0 {
			// Sliding right (showing)
			p.X += p.slideSpeed
			if p.X > p.TargetX {
				p.X = p.TargetX
			}
		} else {
			// Sliding left (hiding)
			p.X -= p.slideSpeed
			if p.X < p.TargetX {
				p.X = p.TargetX
			}
		}

		// Update widget positions during animation
		p.updateWidgetPositions()
	}

	// Handle scroll
	_, dy := ebiten.Wheel()
	if dy != 0 {
		p.ScrollOffset -= dy * 20

		// Clamp scroll
		maxScroll := p.calculateTotalHeight() - p.Height + 40
		if maxScroll < 0 {
			maxScroll = 0
		}
		if p.ScrollOffset < 0 {
			p.ScrollOffset = 0
		}
		if p.ScrollOffset > maxScroll {
			p.ScrollOffset = maxScroll
		}
	}

	// Update all widgets
	for _, widget := range p.Widgets {
		widget.Update()
	}

	// Update hide button (only when panel is fully visible and not animating)
	if !p.IsCollapsed && p.X == p.TargetX {
		p.hideButton.Update()
	}
}

// Draw renders the panel and all widgets
func (p *UIPanel) Draw(screen *ebiten.Image) {
	// Draw panel background
	vector.FillRect(screen,
		float32(p.X), float32(p.Y),
		float32(p.Width), float32(p.Height),
		p.BGColor, true)

	// Draw border
	vector.StrokeRect(screen,
		float32(p.X), float32(p.Y),
		float32(p.Width), float32(p.Height),
		2, p.BorderColor, true)

	// Draw title
	ebitenutil.DebugPrintAt(screen, "Configuration", int(p.X+10), int(p.Y+5))

	// Draw hide button
	p.hideButton.Draw(screen)
	ebitenutil.DebugPrintAt(screen, p.hideButton.Label,
		int(p.hideButton.X+5), int(p.hideButton.Y+3))

	// Draw widgets with clipping and scrolling
	currentY := p.Y + 30 - p.ScrollOffset
	widgetIdx := 0

	for sectionIdx, section := range p.sections {
		// Draw section header
		if currentY >= p.Y-25 && currentY <= p.Y+p.Height {
			sectionBG := color.RGBA{R: 60, G: 60, B: 70, A: 255}
			vector.FillRect(screen,
				float32(p.X+5), float32(currentY),
				float32(p.Width-10), 20,
				sectionBG, true)
			ebitenutil.DebugPrintAt(screen, section.Title,
				int(p.X+10), int(currentY+5))
		}
		currentY += 25

		// Draw widgets in this section
		for widgetIdx < section.EndIndex && widgetIdx < len(p.Widgets) {
			widget := p.Widgets[widgetIdx]
			label := p.Labels[widgetIdx]

			// Only draw if visible
			if currentY >= p.Y-30 && currentY <= p.Y+p.Height {
				// Handle different widget types
				switch w := widget.(type) {
				case *CheckboxWrapper:
					// For checkbox: draw checkbox and label on same line
					p.adjustWidgetPosition(widget, currentY+2)
					widget.Draw(screen)
					// Label to the right of checkbox
					ebitenutil.DebugPrintAt(screen, label,
						int(p.X+10+w.Size+8), int(currentY))

				case *ButtonWrapper:
					// For button: draw button with label centered inside
					p.adjustWidgetPosition(widget, currentY)
					widget.Draw(screen)
					// Label centered in button
					textOffset := (len(label) * 8) / 2
					ebitenutil.DebugPrintAt(screen, label,
						int(p.X+p.Width/2-float64(textOffset)), int(currentY+8))

				default:
					// For sliders: draw label above
					ebitenutil.DebugPrintAt(screen, label,
						int(p.X+10), int(currentY))
					p.adjustWidgetPosition(widget, currentY+15)
					widget.Draw(screen)
				}
			}

			currentY += widget.GetHeight()
			widgetIdx++
		}

		// Move to next section
		if sectionIdx < len(p.sections)-1 {
			widgetIdx = p.sections[sectionIdx+1].StartIndex
		}
	}
}

// adjustWidgetPosition temporarily adjusts widget position for rendering
func (p *UIPanel) adjustWidgetPosition(widget UIWidget, newY float64) {
	switch w := widget.(type) {
	case *SliderWrapper:
		w.Y = newY
	case *CheckboxWrapper:
		w.Y = newY
	case *ButtonWrapper:
		w.Y = newY
	}
}

// updateWidgetPositions updates all widget X positions during slide animation
func (p *UIPanel) updateWidgetPositions() {
	for _, widget := range p.Widgets {
		switch w := widget.(type) {
		case *SliderWrapper:
			w.X = p.X + 10
		case *CheckboxWrapper:
			w.X = p.X + 10
		case *ButtonWrapper:
			w.X = p.X + 10
			w.Width = p.Width - 20
		}
	}

	// Update hide button position
	p.hideButton.X = p.X + p.Width - 30
}

// Toggle slides the panel in or out
func (p *UIPanel) Toggle() {
	if p.IsCollapsed {
		// Show panel
		p.TargetX = 10
		p.IsCollapsed = false
	} else {
		// Hide panel
		p.TargetX = -p.Width - 10
		p.IsCollapsed = true
	}
}

// calculateTotalHeight calculates the total content height
func (p *UIPanel) calculateTotalHeight() float64 {
	height := 30.0 // Title space

	// Add section headers
	height += float64(len(p.sections)) * 25

	// Add all widgets
	for _, widget := range p.Widgets {
		height += widget.GetHeight()
	}

	return height
}

// GetSliderValue gets the value of a slider by index
func (p *UIPanel) GetSliderValue(index int) float64 {
	if index < 0 || index >= len(p.Widgets) {
		return 0
	}

	if sw, ok := p.Widgets[index].(*SliderWrapper); ok {
		return sw.Value
	}
	return 0
}

// GetCheckboxValue gets the value of a checkbox by index
func (p *UIPanel) GetCheckboxValue(index int) bool {
	if index < 0 || index >= len(p.Widgets) {
		return false
	}

	if cw, ok := p.Widgets[index].(*CheckboxWrapper); ok {
		return cw.Value
	}
	return false
}
