package calendar

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
	"github.com/sonroyaalmerol/snry-shell/surfaces/widgets"
)

const (
	panelMargin = 12
	panelWidth  = 320
)

// Calendar is a popup showing a navigable month calendar.
type Calendar struct {
	win     *gtk.ApplicationWindow
	bus     *bus.Bus
	trigger gtk.Widgetter
	root    *gtk.Box
}

// New creates and hides the calendar popup anchored to the given trigger widget.
func New(app *gtk.Application, b *bus.Bus, trigger gtk.Widgetter) *Calendar {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-calendar",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeOnDemand,
		ExclusiveZone: -1,
		Namespace:     "snry-calendar",
	})

	c := &Calendar{win: win, bus: b, trigger: trigger}

	clickBg := gtk.NewBox(gtk.OrientationVertical, 0)
	clickBg.SetHExpand(true)
	clickBg.SetVExpand(true)
	clickGesture := gtk.NewGestureClick()
	clickGesture.SetButton(1)
	clickGesture.SetPropagationLimit(gtk.LimitNone)
	clickGesture.ConnectReleased(func(_ int, _ float64, _ float64) {
		c.Close()
	})
	clickBg.AddController(clickGesture)

	c.build(clickBg)
	win.SetVisible(false)

	layershell.SetMargin(win, layershell.EdgeTop, layershell.BarExclusiveZone+8)

	surfaceutil.AddEscapeToClose(win)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		action, _ := e.Data.(string)
		switch action {
		case "toggle-calendar":
			glib.IdleAdd(func() { c.Toggle() })
		case "toggle-controls", "toggle-notif-center", "toggle-overview":
			if c.win.Visible() {
				glib.IdleAdd(func() { c.win.SetVisible(false) })
			}
		}
	})

	return c
}

func (c *Calendar) build(clickBg *gtk.Box) {
	root := gtk.NewBox(gtk.OrientationHorizontal, 0)
	root.AddCSSClass("popup-overlay")
	root.SetHAlign(gtk.AlignEnd)
	root.SetVAlign(gtk.AlignStart)

	panel := gtk.NewBox(gtk.OrientationVertical, 0)
	panel.AddCSSClass("popup-panel")
	panel.SetMarginStart(panelMargin)
	panel.SetMarginEnd(panelMargin)
	panel.SetSizeRequest(panelWidth, -1)

	panel.Append(widgets.BuildCalendarGroup())

	root.Append(panel)
	clickBg.Append(root)
	c.win.SetChild(clickBg)
	c.root = root
}

func (c *Calendar) Toggle() {
	if c.win.Visible() {
		c.win.SetVisible(false)
	} else {
		c.win.SetVisible(true)
	}
}

func (c *Calendar) Close() {
	c.win.SetVisible(false)
}
