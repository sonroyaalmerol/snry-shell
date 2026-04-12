package network

import (
	"context"

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

// Network is a popup showing the Network connection picker and info.
type Network struct {
	win     *gtk.ApplicationWindow
	bus     *bus.Bus
	refs    *servicerefs.ServiceRefs
	trigger gtk.Widgetter
	monitor *gdk.Monitor
	root    *gtk.Box
	scroll  *gtk.ScrolledWindow
}

// New creates and hides the Network popup anchored to the given trigger widget.
func New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs, trigger gtk.Widgetter) *Network {
	win, _, root := surfaceutil.NewPopupPanel(app, b, surfaceutil.PopupPanelConfig{
		Name:      "snry-network",
		Namespace: "snry-network",
		CloseOn:   []string{"toggle-notif-center", "toggle-bluetooth", "toggle-calendar", "toggle-overview"},
		Align:     gtk.AlignEnd,
	})

	n := &Network{win: win, bus: b, refs: refs, trigger: trigger, root: root}

	panel := gtk.NewBox(gtk.OrientationVertical, 0)
	panel.AddCSSClass("popup-panel")
	panel.SetMarginStart(panelMargin)
	panel.SetMarginEnd(panelMargin)
	panel.SetSizeRequest(panelWidth, -1)

	// Header
	panel.Append(gtkutil.HeaderBar("Network", ""))

	n.scroll = gtk.NewScrolledWindow()
	n.scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	n.scroll.AddCSSClass("popup-scroll")
	n.scroll.SetMaxContentHeight(surfaceutil.PopupMaxHeight(n.monitor, layershell.BarHeight()))
	n.scroll.SetPropagateNaturalHeight(true)
	gtkutil.SetupScrollHoverSuppression(n.scroll)

	n.scroll.SetChild(widgets.NewNetworkWidget(n.bus, refs, n.win))
	panel.Append(n.scroll)
	root.Append(panel)

	b.Subscribe(bus.TopicPopupTrigger, func(e bus.Event) {
		pt, ok := e.Data.(surfaceutil.PopupTrigger)
		if !ok {
			return
		}
		if pt.Action == "toggle-wifi" {
			n.trigger = pt.Trigger
			n.monitor = pt.Monitor
		}
	})

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-wifi" {
			glib.IdleAdd(func() { n.Toggle() })
		}
	})

	return n
}

func (n *Network) Toggle() {
	if n.win.Visible() {
		n.win.SetVisible(false)
	} else {
		if n.monitor != nil {
			layershell.SetMonitor(n.win, n.monitor)
		}
		surfaceutil.PositionUnderTrigger(n.root, n.trigger, panelWidth, panelMargin, n.monitor)
		if n.refs.Network != nil {
			go n.refs.Network.ScanWiFi(context.Background())
		}
		// Update max height based on current monitor and scroll to top when opening
		if n.scroll != nil {
			n.scroll.SetMaxContentHeight(surfaceutil.PopupMaxHeight(n.monitor, layershell.BarHeight()))
			n.scroll.SetVAdjustment(gtk.NewAdjustment(0, 0, 0, 0, 0, 0))
		}
		n.win.SetVisible(true)
	}
}
