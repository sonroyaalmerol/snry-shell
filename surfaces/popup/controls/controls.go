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

const (
	panelMargin = 12
	panelWidth  = 350
)

// Controls is a popup dialog showing volume, brightness, and wallpaper controls.
type Controls struct {
	win     *gtk.ApplicationWindow
	bus     *bus.Bus
	trigger gtk.Widgetter
	root    *gtk.Box
}

// New creates and hides the controls popup anchored to the given trigger widget.
func New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs, trigger gtk.Widgetter) *Controls {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-controls",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeOnDemand,
		ExclusiveZone: -1,
		Namespace:     "snry-controls",
	})

	c := &Controls{win: win, bus: b, trigger: trigger}
	c.build(refs)
	win.SetVisible(false)

	// Layer-shell top margin keeps the window below the bar.
	layershell.SetMargin(win, layershell.EdgeTop, layershell.BarExclusiveZone+8)

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

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("popup-scroll")
	scroll.SetMaxContentHeight(500)
	scroll.SetPropagateNaturalHeight(true)

	panel := gtk.NewBox(gtk.OrientationVertical, 8)
	panel.AddCSSClass("popup-panel")
	panel.SetMarginStart(panelMargin)
	panel.SetMarginEnd(panelMargin)
	panel.SetSizeRequest(panelWidth, -1)

	panel.Append(widgets.BuildQuickControls(c.bus, refs))

	scroll.SetChild(panel)
	root.Append(scroll)
	c.win.SetChild(root)
	c.root = root
}

func (c *Controls) positionUnderTrigger() {
	triggerX := surfaceutil.WidgetXRelativeToRoot(c.trigger)
	triggerW := surfaceutil.WidgetWidth(c.trigger)
	popupW := panelWidth + panelMargin*2
	monW := surfaceutil.MonitorWidth()

	desiredLeft := triggerX + triggerW/2 - popupW/2
	if monW > 0 {
		if desiredLeft < panelMargin {
			desiredLeft = panelMargin
		}
		if desiredLeft+popupW > monW-panelMargin {
			desiredLeft = monW - panelMargin - popupW
		}
	}

	c.root.SetMarginStart(desiredLeft)
}

func (c *Controls) Toggle() {
	if c.win.Visible() {
		c.win.SetVisible(false)
	} else {
		c.positionUnderTrigger()
		c.win.SetVisible(true)
	}
}

func (c *Controls) Close() {
	c.win.SetVisible(false)
}
