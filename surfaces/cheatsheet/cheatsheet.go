// Package cheatsheet provides a keyboard shortcuts overlay.
package cheatsheet

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

// Keybind represents a single keyboard shortcut entry.
type Keybind struct {
	Keys        string
	Description string
}

var defaultKeybinds = []Keybind{
	{"Super + Enter", "Open terminal"},
	{"Super + Q", "Close window"},
	{"Super + Space", "Open overview / launcher"},
	{"Super + Tab", "Cycle windows"},
	{"Super + 1–9", "Switch to workspace"},
	{"Super + Shift + 1–9", "Move window to workspace"},
	{"Super + F", "Toggle fullscreen"},
	{"Super + V", "Toggle floating"},
	{"Super + H/J/K/L", "Move focus"},
	{"Super + Shift + H/J/K/L", "Move window"},
	{"Super + R", "Resize mode"},
	{"Print", "Screenshot region"},
	{"Shift + Print", "Screenshot full"},
}

// Cheatsheet is an overlay showing keybindings.
type Cheatsheet struct {
	win *gtk.ApplicationWindow
	bus *bus.Bus
}

func New(app *gtk.Application, b *bus.Bus) *Cheatsheet {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:         "snry-cheatsheet",
		Layer:        layershell.LayerOverlay,
		Anchors:      layershell.FullscreenAnchors(),
		KeyboardMode: layershell.KeyboardModeOnDemand,
		Namespace:    "snry-cheatsheet",
	})

	cs := &Cheatsheet{win: win, bus: b}
	cs.build()
	win.SetVisible(false)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-cheatsheet" {
			glib.IdleAdd(func() { cs.Toggle() })
		}
	})

	return cs
}

func (cs *Cheatsheet) build() {
	outer := gtk.NewBox(gtk.OrientationVertical, 0)
	outer.AddCSSClass("cheatsheet")
	outer.SetHAlign(gtk.AlignCenter)
	outer.SetVAlign(gtk.AlignCenter)

	card := gtk.NewBox(gtk.OrientationVertical, 8)
	card.AddCSSClass("cheatsheet-card")
	card.SetMarginTop(24)
	card.SetMarginBottom(24)
	card.SetMarginStart(32)
	card.SetMarginEnd(32)

	title := gtk.NewLabel("Keyboard Shortcuts")
	title.AddCSSClass("cheatsheet-title")
	card.Append(title)

	grid := gtk.NewGrid()
	grid.SetColumnSpacing(32)
	grid.SetRowSpacing(4)
	for i, kb := range defaultKeybinds {
		keys := gtk.NewLabel(kb.Keys)
		keys.AddCSSClass("cheatsheet-key")
		keys.SetHAlign(gtk.AlignEnd)

		desc := gtk.NewLabel(kb.Description)
		desc.AddCSSClass("cheatsheet-description")
		desc.SetHAlign(gtk.AlignStart)

		grid.Attach(keys, 0, i, 1, 1)
		grid.Attach(desc, 1, i, 1, 1)
	}
	card.Append(grid)
	outer.Append(card)

	// Close on Escape or click outside the card.
	surfaceutil.AddEscapeToClose(cs.win)
	cs.win.SetChild(outer)
}

// Toggle shows or hides the cheatsheet.
func (cs *Cheatsheet) Toggle() {
	cs.win.SetVisible(!cs.win.Visible())
}
