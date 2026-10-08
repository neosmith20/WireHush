//go:build windows

package ui

import (
	"github.com/lxn/walk"
)

// darkButton is the small owner-drawn action primitive used anywhere a native
// PushButton would otherwise introduce a light rectangle into the dashboard.
type darkButton struct {
	*walk.CustomWidget
	text    string
	primary bool
	enabled bool
	clicked walk.EventPublisher
	font    *walk.Font
}

func newDarkButton(parent walk.Container, text string, primary bool) (*darkButton, error) {
	button := &darkButton{text: text, primary: primary, enabled: true}
	// Walk may request an initial paint while NewCustomWidgetPixels is still
	// constructing the HWND.  Every paint dependency must therefore exist
	// before creating the widget.
	button.font, _ = walk.NewFont("Segoe UI Semibold", 10, 0)
	widget, err := walk.NewCustomWidgetPixels(parent, 0, button.paint)
	if err != nil {
		return nil, err
	}
	button.CustomWidget = widget
	button.SetPaintMode(walk.PaintBuffered)
	button.SetInvalidatesOnResize(true)
	button.SetMinMaxSize(walk.Size{120, 36}, walk.Size{0, 36})
	button.MouseUp().Attach(func(_ int, _ int, mouse walk.MouseButton) {
		if mouse == walk.LeftButton && button.Enabled() {
			button.clicked.Publish()
		}
	})
	return button, nil
}

func (button *darkButton) Clicked() *walk.Event { return button.clicked.Event() }
func (button *darkButton) SetText(text string)  { button.text = text; button.Invalidate() }
func (button *darkButton) SetEnabled(enabled bool) {
	button.enabled = enabled
	if button.CustomWidget != nil {
		button.CustomWidget.SetEnabled(enabled)
		button.Invalidate()
	}
}

func (button *darkButton) paint(canvas *walk.Canvas, bounds walk.Rectangle) error {
	brush := uiCardBrush
	color := uiTextColor
	// Paint can run during NewCustomWidgetPixels before the embedded widget is
	// assigned, so presentation must not query its HWND-backed state here.
	if button.primary && button.enabled {
		brush, color = uiAccentBrush, walk.RGB(4, 20, 28)
	} else if !button.enabled {
		color = uiMutedColor
	}
	canvas.FillRectangle(brush, bounds)
	return canvas.DrawTextPixels(button.text, button.font, color, bounds, walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)
}
