// Package overview provides the full-screen application launcher and window
// preview grid.
package overview

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
)

// Overview is a full-screen overlay showing window previews and the launcher.
type Overview struct {
	win     *gtk.ApplicationWindow
	bus     *bus.Bus
	querier *hyprland.Querier
	grid    *gridWidget
}

func New(app *gtk.Application, b *bus.Bus, querier *hyprland.Querier) *Overview {
	win := gtk.NewApplicationWindow(app)
	win.SetDecorated(false)
	win.SetName("snry-overview")

	layershell.InitForWindow(win)
	layershell.SetLayer(win, layershell.LayerOverlay)
	layershell.SetAnchor(win, layershell.EdgeTop, true)
	layershell.SetAnchor(win, layershell.EdgeBottom, true)
	layershell.SetAnchor(win, layershell.EdgeLeft, true)
	layershell.SetAnchor(win, layershell.EdgeRight, true)
	layershell.SetKeyboardMode(win, layershell.KeyboardModeOnDemand)
	layershell.SetNamespace(win, "snry-overview")

	ov := &Overview{win: win, bus: b, querier: querier}
	ov.build()

	// Hidden by default; toggled by control socket or keybind.
	win.SetVisible(false)

	ov.bus.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-overview" {
			glib.IdleAdd(func() {
				ov.Toggle()
				if ov.win.Visible() {
					ov.win.GrabFocus()
				}
			})
		}
	})

	return ov
}

func (o *Overview) build() {
	root := gtk.NewBox(gtk.OrientationVertical, 12)
	root.AddCSSClass("overview")
	root.SetMarginTop(48)
	root.SetMarginBottom(48)
	root.SetMarginStart(48)
	root.SetMarginEnd(48)

	root.Append(newSearchWidget(o.bus, o.hide))
	gw := newGridWidget(o.bus, o.querier)
	o.grid = gw
	root.Append(gw.scroll)

	// Dismiss on Escape.
	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.ConnectKeyPressed(func(keyval, keycode uint, state gdk.ModifierType) bool {
		if keyval == 0xff1b { // GDK_KEY_Escape
			o.hide()
			return true
		}
		return false
	})
	o.win.AddController(keyCtrl)
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
