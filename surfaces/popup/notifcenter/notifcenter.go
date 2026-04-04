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

// NotifCenter is a popup dialog showing notifications, WiFi, and Bluetooth.
type NotifCenter struct {
	win     *gtk.ApplicationWindow
	bus     *bus.Bus
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

	nc := &NotifCenter{win: win, bus: b, trigger: trigger}
	nc.build(refs)
	win.SetVisible(false)

	clickGesture := gtk.NewGestureClick()
	clickGesture.SetButton(1)
	clickGesture.ConnectReleased(func(_ int, _ float64, _ float64) {
		nc.Close()
	})
	win.AddController(clickGesture)

	surfaceutil.AddEscapeToClose(win)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		action, _ := e.Data.(string)
		switch action {
		case "toggle-notif-center":
			glib.IdleAdd(func() { nc.Toggle() })
		case "toggle-controls", "toggle-calendar-media":
			if nc.win.Visible() {
				glib.IdleAdd(func() { nc.win.SetVisible(false) })
			}
		}
	})

	return nc
}

func (nc *NotifCenter) build(refs *servicerefs.ServiceRefs) {
	root := gtk.NewBox(gtk.OrientationHorizontal, 0)
	root.AddCSSClass("popup-overlay")
	root.SetHAlign(gtk.AlignStart)
	root.SetVAlign(gtk.AlignStart)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("popup-scroll")
	scroll.SetMaxContentHeight(600)
	scroll.SetPropagateNaturalHeight(true)

	panel := gtk.NewBox(gtk.OrientationVertical, 8)
	panel.AddCSSClass("popup-panel")
	panel.SetMarginStart(panelMargin)
	panel.SetMarginEnd(panelMargin)
	panel.SetSizeRequest(panelWidth, -1)

	panel.Append(widgets.NewNotificationList(nc.bus))
	panel.Append(widgets.NewWiFiWidget(nc.bus, refs))
	panel.Append(widgets.NewBluetoothWidget(nc.bus, refs))

	scroll.SetChild(panel)
	root.Append(scroll)
	nc.win.SetChild(root)
	nc.root = root
}

func (nc *NotifCenter) positionUnderTrigger() {
	monW := surfaceutil.MonitorWidth()
	if monW <= 0 {
		return
	}

	triggerX := surfaceutil.WidgetXRelativeToRoot(nc.trigger)
	triggerW := surfaceutil.WidgetWidth(nc.trigger)
	popupW := panelWidth + panelMargin*2

	desiredLeft := triggerX + triggerW/2 - popupW/2
	if desiredLeft < panelMargin {
		desiredLeft = panelMargin
	}
	if desiredLeft+popupW > monW-panelMargin {
		desiredLeft = monW - panelMargin - popupW
	}

	nc.root.SetMarginTop(layershell.BarExclusiveZone + 8)
	nc.root.SetMarginStart(desiredLeft)
}

func (nc *NotifCenter) Toggle() {
	if nc.win.Visible() {
		nc.win.SetVisible(false)
	} else {
		nc.positionUnderTrigger()
		nc.win.SetVisible(true)
	}
}

func (nc *NotifCenter) Close() {
	nc.win.SetVisible(false)
}
