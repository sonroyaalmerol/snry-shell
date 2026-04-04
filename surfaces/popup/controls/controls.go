package controls

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
	"github.com/sonroyaalmerol/snry-shell/surfaces/widgets"
)

// Controls is a popup dialog showing volume, brightness, and wallpaper controls.
type Controls struct {
	win *gtk.ApplicationWindow
	bus *bus.Bus
}

// New creates and hides the controls popup.
func New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs) *Controls {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-controls",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeOnDemand,
		ExclusiveZone: -1,
		Namespace:     "snry-controls",
	})

	c := &Controls{win: win, bus: b}
	c.build(refs)
	win.SetVisible(false)

	// Click background to close.
	clickGesture := gtk.NewGestureClick()
	clickGesture.SetButton(1)
	clickGesture.ConnectReleased(func(_ int, _ float64, _ float64) {
		c.Close()
	})
	win.AddController(clickGesture)

	surfaceutil.AddEscapeToClose(win)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		action, _ := e.Data.(string)
		switch action {
		case "toggle-controls":
			glib.IdleAdd(func() { c.Toggle() })
		case "toggle-calendar-media", "toggle-notif-center":
			if c.win.Visible() {
				glib.IdleAdd(func() { c.win.SetVisible(false) })
			}
		}
	})

	return c
}

func (c *Controls) build(refs *servicerefs.ServiceRefs) {
	root := gtk.NewBox(gtk.OrientationHorizontal, 0)
	root.AddCSSClass("popup-overlay")
	root.SetHAlign(gtk.AlignStart)
	root.SetVAlign(gtk.AlignStart)
	root.SetMarginTop(layershell.BarExclusiveZone + 8)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("popup-scroll")
	scroll.SetMaxContentHeight(500)
	scroll.SetPropagateNaturalHeight(true)

	panel := gtk.NewBox(gtk.OrientationVertical, 8)
	panel.AddCSSClass("popup-panel")
	panel.SetMarginStart(12)
	panel.SetMarginEnd(12)

	panel.Append(widgets.BuildQuickControls(c.bus, refs))

	scroll.SetChild(panel)
	root.Append(scroll)
	c.win.SetChild(root)
}

func (c *Controls) Toggle() {
	c.win.SetVisible(!c.win.Visible())
}

func (c *Controls) Close() {
	c.win.SetVisible(false)
}
