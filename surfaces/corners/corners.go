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

// Corners holds the windows for one monitor's corner hotspots.
type Corners struct {
	windows []*gtk.ApplicationWindow
}

// Close destroys all corner windows.
func (c *Corners) Close() {
	for _, w := range c.windows {
		w.Close()
	}
}

// New creates four invisible corner hotspots on the given monitor.
func New(app *gtk.Application, b *bus.Bus, mon *gdk.Monitor) *Corners {
	geom := mon.Geometry()

	actions := map[CornerPosition]string{
		TopLeft:     "toggle-overview",
		TopRight:    "toggle-notif-center",
		BottomLeft:  "toggle-session",
		BottomRight: "toggle-media-overlay",
	}

	positions := map[CornerPosition][2]int{
		TopLeft:     {0, 0},
		TopRight:    {int(geom.Width()) - cornerSize, 0},
		BottomLeft:  {0, int(geom.Height()) - cornerSize},
		BottomRight: {int(geom.Width()) - cornerSize, int(geom.Height()) - cornerSize},
	}

	c := &Corners{}

	for pos, action := range actions {
		win := layershell.NewWindow(app, layershell.WindowConfig{
			Name:          "snry-corner",
			Layer:         layershell.LayerOverlay,
			Anchors:       layershell.FullscreenAnchors(),
			KeyboardMode:  layershell.KeyboardModeNone,
			ExclusiveZone: -1,
			Namespace:     "snry-corner",
			Monitor:       mon,
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

		c.windows = append(c.windows, win)
	}

	return c
}
