// Package mediaoverlay provides a full-screen media player controls overlay.
package mediaoverlay

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/services/mpris"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
	"github.com/sonroyaalmerol/snry-shell/surfaces/widgets"
)

// New creates a full-screen media overlay surface.
func New(app *gtk.Application, b *bus.Bus, mprisSvc *mpris.Service) {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-media-overlay",
		Layer:         layershell.LayerTop,
		Anchors:       map[layershell.Edge]bool{layershell.EdgeTop: true, layershell.EdgeRight: true},
		Margins:       map[layershell.Edge]int{layershell.EdgeTop: 44, layershell.EdgeRight: 8},
		KeyboardMode:  layershell.KeyboardModeNone,
		ExclusiveZone: -1,
		Namespace:     "snry-media-overlay",
	})

	root := gtk.NewBox(gtk.OrientationVertical, 0)
	root.Append(widgets.BuildMediaGroupWithPrefix(b, mprisSvc, "media-overlay-"))
	win.SetChild(root)

	surfaceutil.AddToggleOn(b, win, "toggle-media-overlay")
	win.SetVisible(false)
}
