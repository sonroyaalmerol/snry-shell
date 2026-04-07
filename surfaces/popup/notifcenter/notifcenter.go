package notifcenter

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
	"github.com/sonroyaalmerol/snry-shell/surfaces/widgets"
)

const (
	panelMargin = 12
	panelWidth  = 360
)

// NotifCenter is a popup showing notifications and media controls.
type NotifCenter struct {
	win     *gtk.ApplicationWindow
	bus     *bus.Bus
	refs    *servicerefs.ServiceRefs
	trigger gtk.Widgetter
	monitor *gdk.Monitor
	root    *gtk.Box
	scroll  *gtk.ScrolledWindow
}

// New creates and hides the notification center popup anchored to the given trigger widget.
func New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs, trigger gtk.Widgetter) *NotifCenter {
	win, _, root := surfaceutil.NewPopupPanel(app, b, surfaceutil.PopupPanelConfig{
		Name:      "snry-notif-center",
		Namespace: "snry-notif-center",
		CloseOn:   []string{"toggle-wifi", "toggle-bluetooth", "toggle-calendar", "toggle-overview"},
		Align:     gtk.AlignEnd,
	})

	nc := &NotifCenter{win: win, bus: b, refs: refs, trigger: trigger, root: root}

	panel := gtk.NewBox(gtk.OrientationVertical, 0)
	panel.AddCSSClass("popup-panel")
	panel.SetMarginStart(panelMargin)
	panel.SetMarginEnd(panelMargin)
	panel.SetSizeRequest(panelWidth, -1)

	// Header
	header := gtk.NewLabel("Notifications")
	header.AddCSSClass("popup-header")
	header.SetHAlign(gtk.AlignStart)
	panel.Append(header)

	nc.scroll = gtk.NewScrolledWindow()
	nc.scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	nc.scroll.AddCSSClass("popup-scroll")
	nc.scroll.SetMaxContentHeight(surfaceutil.PopupMaxHeight(nc.monitor, layershell.BarHeight()))
	nc.scroll.SetPropagateNaturalHeight(true)
	gtkutil.SetupScrollHoverSuppression(nc.scroll)

	content := gtk.NewBox(gtk.OrientationVertical, 8)
	content.Append(widgets.NewNotificationList(nc.bus))
	content.Append(gtkutil.M3Divider())
	content.Append(widgets.BuildMediaGroup(nc.bus, nc.refs.Mpris))

	nc.scroll.SetChild(content)
	panel.Append(nc.scroll)
	root.Append(panel)

	b.Subscribe(bus.TopicPopupTrigger, func(e bus.Event) {
		pt, ok := e.Data.(surfaceutil.PopupTrigger)
		if !ok {
			return
		}
		if pt.Action == "toggle-notif-center" {
			nc.trigger = pt.Trigger
			nc.monitor = pt.Monitor
		}
	})

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-notif-center" {
			glib.IdleAdd(func() { nc.Toggle() })
		}
	})

	return nc
}

func (nc *NotifCenter) Toggle() {
	if nc.win.Visible() {
		nc.win.SetVisible(false)
	} else {
		if nc.monitor != nil {
			layershell.SetMonitor(nc.win, nc.monitor)
		}
		surfaceutil.PositionUnderTrigger(nc.root, nc.trigger, panelWidth, panelMargin, nc.monitor)
		// Update max height based on current monitor and scroll to top when opening
		if nc.scroll != nil {
			nc.scroll.SetMaxContentHeight(surfaceutil.PopupMaxHeight(nc.monitor, layershell.BarHeight()))
			nc.scroll.SetVAdjustment(gtk.NewAdjustment(0, 0, 0, 0, 0, 0))
		}
		nc.win.SetVisible(true)
	}
}

