// Package imageviewer provides a floating overlay for viewing images.
package imageviewer

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

// Viewer is a centered overlay that displays a single image.
type Viewer struct {
	win *gtk.ApplicationWindow
	bus *bus.Bus
}

func New(app *gtk.Application, b *bus.Bus) *Viewer {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-image-viewer",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeOnDemand,
		ExclusiveZone: -1,
		Namespace:     "snry-image-viewer",
	})

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

	surfaceutil.AddEscapeToClose(win)
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
