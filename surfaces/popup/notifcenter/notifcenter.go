package notifcenter

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
	panelWidth  = 420
)

// NotifCenter is a unified popup showing quick settings, WiFi, Bluetooth,
// notifications, media, and calendar.
type NotifCenter struct {
	win     *gtk.ApplicationWindow
	bus     *bus.Bus
	refs    *servicerefs.ServiceRefs
	trigger gtk.Widgetter
	root    *gtk.Box
}

// New creates and hides the notification center popup anchored to the given trigger widget.
func New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs, trigger gtk.Widgetter) *NotifCenter {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-notif-center",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeOnDemand,
		ExclusiveZone: -1,
		Namespace:     "snry-notif-center",
	})

	nc := &NotifCenter{win: win, bus: b, refs: refs, trigger: trigger}

	// Full-window background that catches clicks outside the panel.
	clickBg := gtk.NewBox(gtk.OrientationVertical, 0)
	clickBg.SetHExpand(true)
	clickBg.SetVExpand(true)
	clickGesture := gtk.NewGestureClick()
	clickGesture.SetButton(1)
	clickGesture.SetPropagationLimit(gtk.LimitNone)
	clickGesture.ConnectReleased(func(_ int, _ float64, _ float64) {
		nc.Close()
	})
	clickBg.AddController(clickGesture)

	nc.build(refs, clickBg)
	win.SetVisible(false)

	layershell.SetMargin(win, layershell.EdgeTop, layershell.BarExclusiveZone+8)

	surfaceutil.AddEscapeToClose(win)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		action, _ := e.Data.(string)
		switch action {
		case "toggle-notif-center":
			glib.IdleAdd(func() { nc.Toggle() })
		case "toggle-controls", "toggle-overview", "toggle-calendar":
			if nc.win.Visible() {
				glib.IdleAdd(func() { nc.win.SetVisible(false) })
			}
		}
	})

	return nc
}

func (nc *NotifCenter) build(refs *servicerefs.ServiceRefs, clickBg *gtk.Box) {
	root := gtk.NewBox(gtk.OrientationHorizontal, 0)
	root.AddCSSClass("popup-overlay")
	root.SetHAlign(gtk.AlignStart)
	root.SetVAlign(gtk.AlignStart)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("popup-scroll")
	scroll.SetMaxContentHeight(800)
	scroll.SetPropagateNaturalHeight(true)

	panel := gtk.NewBox(gtk.OrientationVertical, 8)
	panel.AddCSSClass("popup-panel")
	panel.SetMarginStart(panelMargin)
	panel.SetMarginEnd(panelMargin)
	panel.SetSizeRequest(panelWidth, -1)

	// Top: quick settings, WiFi, Bluetooth.
	panel.Append(widgets.NewQuickToggles(nc.bus, refs))
	panel.Append(widgets.NewWiFiWidget(nc.bus, refs, nc.win))
	panel.Append(widgets.NewBluetoothWidget(nc.bus, refs, nc.win))

	// Separator.
	sep := gtk.NewBox(gtk.OrientationHorizontal, 0)
	sep.AddCSSClass("popup-separator")
	sep.SetMarginTop(4)
	sep.SetMarginBottom(4)
	panel.Append(sep)

	// Middle: notifications, media, calendar.
	panel.Append(widgets.NewNotificationList(nc.bus))
	panel.Append(widgets.BuildMediaGroup(nc.bus, refs.Mpris))

	scroll.SetChild(panel)
	root.Append(scroll)
	clickBg.Append(root)
	nc.win.SetChild(clickBg)
	nc.root = root
}

func (nc *NotifCenter) positionUnderTrigger() {
	triggerX := surfaceutil.WidgetXRelativeToRoot(nc.trigger)
	triggerW := surfaceutil.WidgetWidth(nc.trigger)
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

	nc.root.SetMarginStart(desiredLeft)
}

func (nc *NotifCenter) Toggle() {
	if nc.win.Visible() {
		nc.win.SetVisible(false)
	} else {
		nc.positionUnderTrigger()
		if nc.refs.Network != nil {
			go nc.refs.Network.ScanWiFi()
		}
		nc.win.SetVisible(true)
	}
}

func (nc *NotifCenter) Close() {
	nc.win.SetVisible(false)
}
