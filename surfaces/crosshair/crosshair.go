// Package crosshair provides a full-screen crosshair overlay.
package crosshair

import (
	"math"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
)

// Crosshair is a transparent full-screen overlay drawing a crosshair at center.
type Crosshair struct {
	win *gtk.ApplicationWindow
	bus *bus.Bus
}

func New(app *gtk.Application, b *bus.Bus) *Crosshair {
	win := gtk.NewApplicationWindow(app)
	win.SetDecorated(false)
	win.SetName("snry-crosshair")

	layershell.InitForWindow(win)
	layershell.SetLayer(win, layershell.LayerOverlay)
	layershell.SetAnchor(win, layershell.EdgeTop, true)
	layershell.SetAnchor(win, layershell.EdgeBottom, true)
	layershell.SetAnchor(win, layershell.EdgeLeft, true)
	layershell.SetAnchor(win, layershell.EdgeRight, true)
	layershell.SetKeyboardMode(win, layershell.KeyboardModeNone)
	layershell.SetExclusiveZone(win, -1)
	layershell.SetNamespace(win, "snry-crosshair")

	area := gtk.NewDrawingArea()
	area.AddCSSClass("crosshair-overlay")
	area.SetHExpand(true)
	area.SetVExpand(true)

	area.SetDrawFunc(func(da *gtk.DrawingArea, ctx *cairo.Context, w, h int) {
		cx := float64(w) / 2
		cy := float64(h) / 2

		ctx.SetSourceRGBA(1, 0, 0, 0.85)
		ctx.SetLineWidth(1.5)

		// Horizontal line.
		ctx.MoveTo(cx-20, cy)
		ctx.LineTo(cx+20, cy)
		ctx.Stroke()

		// Vertical line.
		ctx.MoveTo(cx, cy-20)
		ctx.LineTo(cx, cy+20)
		ctx.Stroke()

		// Center circle.
		ctx.SetLineWidth(1)
		ctx.Arc(cx, cy, 6, 0, 2*math.Pi)
		ctx.Stroke()
	})

	win.SetChild(area)

	c := &Crosshair{win: win, bus: b}

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-crosshair" {
			win.SetVisible(!win.Visible())
		}
	})

	win.SetVisible(false)
	return c
}
