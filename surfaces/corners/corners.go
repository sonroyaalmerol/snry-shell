// Package corners provides invisible screen-corner hotspots that trigger
// actions on mouse enter.
package corners

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
)

const cornerSize = 1

type CornerPosition int

const (
	TopLeft CornerPosition = iota
	TopRight
	BottomLeft
	BottomRight
)

func New(app *gtk.Application, b *bus.Bus) {
	display := gdk.DisplayGetDefault()
	if display == nil {
		return
	}

	monitors := display.Monitors()
	if monitors.NItems() == 0 {
		return
	}

	// Use MonitorAtSurface requires a surface. Instead, build a temporary
	// offscreen surface to get the primary monitor geometry, or use the
	// monitor from the list directly via casting through the glib type system.
	// Since Monitor embeds *coreglib.Object and ListModel.Item returns *coreglib.Object,
	// we can construct a Monitor from the Object pointer.
	item := monitors.Item(0)
	if item == nil {
		return
	}

	mon := &gdk.Monitor{Object: item}
	geom := mon.Geometry()

	actions := map[CornerPosition]string{
		TopLeft:     "toggle-overview",
		TopRight:    "toggle-sidebar",
		BottomLeft:  "toggle-session",
		BottomRight: "toggle-media-overlay",
	}

	positions := map[CornerPosition][2]int{
		TopLeft:     {0, 0},
		TopRight:    {int(geom.Width()) - cornerSize, 0},
		BottomLeft:  {0, int(geom.Height()) - cornerSize},
		BottomRight: {int(geom.Width()) - cornerSize, int(geom.Height()) - cornerSize},
	}

	for pos, action := range actions {
		win := layershell.NewWindow(app, layershell.WindowConfig{
			Name:          "snry-corner",
			Layer:         layershell.LayerOverlay,
			Anchors:       layershell.FullscreenAnchors(),
			KeyboardMode:  layershell.KeyboardModeNone,
			ExclusiveZone: -1,
			Namespace:     "snry-corner",
		})

		box := gtk.NewBox(gtk.OrientationHorizontal, 0)
		box.SetSizeRequest(cornerSize, cornerSize)
		box.AddCSSClass("corner-hotspot")

		win.SetChild(box)

		xy := positions[pos]
		layershell.SetMargin(win, layershell.EdgeTop, xy[1])
		layershell.SetMargin(win, layershell.EdgeLeft, xy[0])

		ctrl := gtk.NewEventControllerMotion()
		a := action
		ctrl.ConnectEnter(func(x, y float64) {
			b.Publish(bus.TopicSystemControls, a)
		})
		box.AddController(ctrl)
	}
}
