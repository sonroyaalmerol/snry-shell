// Package imageviewer provides a floating overlay for viewing images.
package imageviewer

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
)

// Viewer is a centered overlay that displays a single image.
type Viewer struct {
	win *gtk.ApplicationWindow
	bus *bus.Bus
}

func New(app *gtk.Application, b *bus.Bus) *Viewer {
	win := gtk.NewApplicationWindow(app)
	win.SetDecorated(false)
	win.SetName("snry-image-viewer")

	layershell.InitForWindow(win)
	layershell.SetLayer(win, layershell.LayerOverlay)
	layershell.SetAnchor(win, layershell.EdgeTop, true)
	layershell.SetAnchor(win, layershell.EdgeBottom, true)
	layershell.SetAnchor(win, layershell.EdgeLeft, true)
	layershell.SetAnchor(win, layershell.EdgeRight, true)
	layershell.SetKeyboardMode(win, layershell.KeyboardModeOnDemand)
	layershell.SetExclusiveZone(win, -1)
	layershell.SetNamespace(win, "snry-image-viewer")

	v := &Viewer{win: win, bus: b}

	// Listen for floating image events.
	b.Subscribe(bus.TopicFloatingImage, func(e bus.Event) {
		if path, ok := e.Data.(string); ok && path != "" {
			v.show(path)
		}
	})

	// Click anywhere to dismiss.
	gesture := gtk.NewGestureClick()
	gesture.ConnectReleased(func(nPress int, x, y float64) {
		win.SetVisible(false)
	})
	win.AddController(gesture)

	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.ConnectKeyPressed(func(keyval, keycode uint, state gdk.ModifierType) bool {
		if keyval == 0xff1b {
			win.SetVisible(false)
			return true
		}
		return false
	})
	win.AddController(keyCtrl)
	win.SetVisible(false)
	return v
}

func (v *Viewer) show(path string) {
	pic := gtk.NewPictureForFilename(path)
	pic.SetContentFit(gtk.ContentFitContain)
	pic.SetCanShrink(true)
	v.win.SetChild(pic)
	v.win.SetVisible(true)
}
