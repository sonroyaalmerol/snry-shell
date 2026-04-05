package controls

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
	"github.com/sonroyaalmerol/snry-shell/surfaces/widgets"
)

const (
	panelMargin = 12
	panelWidth  = 350
)

// Controls is a popup dialog showing volume and brightness controls.
type Controls struct {
	win     *gtk.ApplicationWindow
	bus     *bus.Bus
	trigger gtk.Widgetter
	root    *gtk.Box
}

// New creates and hides the controls popup anchored to the given trigger widget.
func New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs, trigger gtk.Widgetter) *Controls {
	win, _, root := surfaceutil.NewPopupPanel(app, b, surfaceutil.PopupPanelConfig{
		Name:      "snry-controls",
		Namespace: "snry-controls",
		CloseOn:   []string{"toggle-notif-center", "toggle-calendar"},
		Align:     gtk.AlignStart,
	})

	c := &Controls{win: win, bus: b, trigger: trigger, root: root}

	panel := gtk.NewBox(gtk.OrientationVertical, 8)
	panel.AddCSSClass("popup-panel")
	panel.SetMarginStart(panelMargin)
	panel.SetMarginEnd(panelMargin)
	panel.SetSizeRequest(panelWidth, -1)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("popup-scroll")
	scroll.SetMaxContentHeight(500)
	scroll.SetPropagateNaturalHeight(true)

	scroll.SetChild(widgets.BuildQuickControls(c.bus, refs))
	panel.Append(scroll)
	root.Append(panel)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-controls" {
			glib.IdleAdd(func() { c.Toggle() })
		}
	})

	return c
}

func (c *Controls) Toggle() {
	if c.win.Visible() {
		c.win.SetVisible(false)
	} else {
		surfaceutil.PositionUnderTrigger(c.root, c.trigger, panelWidth, panelMargin)
		c.win.SetVisible(true)
	}
}
