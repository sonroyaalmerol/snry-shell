// Package overview provides the full-screen application launcher and window
// preview grid.
package overview

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

// Overview is a full-screen overlay showing window previews and the launcher.
type Overview struct {
	win     *gtk.ApplicationWindow
	querier *hyprland.Querier
	grid    *gridWidget
}

func New(app *gtk.Application, b *bus.Bus, querier *hyprland.Querier) *Overview {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:         "snry-overview",
		Layer:        layershell.LayerTop,
		Anchors:      layershell.FullscreenAnchors(),
		KeyboardMode: layershell.KeyboardModeOnDemand,
		Namespace:    "snry-overview",
	})

	ov := &Overview{win: win, querier: querier}
	ov.build(b)

	// Hidden by default; toggled by control socket or keybind.
	win.SetVisible(false)

	surfaceutil.AddToggleOnWithCallback(win, b, "toggle-overview", func() {
		ov.Toggle()
		if ov.win.Visible() {
			ov.win.GrabFocus()
		}
	})

	return ov
}

func (o *Overview) build(b *bus.Bus) {
	root := gtk.NewBox(gtk.OrientationVertical, 12)
	root.AddCSSClass("overview")
	root.SetMarginTop(48)
	root.SetMarginBottom(48)
	root.SetMarginStart(48)
	root.SetMarginEnd(48)

	root.Append(newSearchWidget(b, o.hide))
	gw := newGridWidget(b, o.querier)
	o.grid = gw
	root.Append(gw.scroll)

	// Dismiss on Escape.
	surfaceutil.AddEscapeToCloseWithCallback(o.win, o.hide)
	o.win.SetChild(root)
}

// Toggle shows or hides the overview.
func (o *Overview) Toggle() {
	if o.win.Visible() {
		o.hide()
	} else {
		o.win.SetVisible(true)
		o.win.GrabFocus()
		o.grid.refresh()
	}
}

func (o *Overview) hide() {
	o.win.SetVisible(false)
}
