package calendarmedia

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

// CalendarMedia is a popup dialog showing media controls, calendar, and quick toggles.
type CalendarMedia struct {
	win     *gtk.ApplicationWindow
	bus     *bus.Bus
	trigger gtk.Widgetter
	root    *gtk.Box
}

// New creates and hides the calendar/media popup anchored to the given trigger widget.
func New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs, trigger gtk.Widgetter) *CalendarMedia {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-calendar-media",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeOnDemand,
		ExclusiveZone: -1,
		Namespace:     "snry-calendar-media",
	})

	cm := &CalendarMedia{win: win, bus: b, trigger: trigger}
	cm.build(refs)
	win.SetVisible(false)

	clickGesture := gtk.NewGestureClick()
	clickGesture.SetButton(1)
	clickGesture.ConnectReleased(func(_ int, _ float64, _ float64) {
		cm.Close()
	})
	win.AddController(clickGesture)

	surfaceutil.AddEscapeToClose(win)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		action, _ := e.Data.(string)
		switch action {
		case "toggle-calendar-media":
			glib.IdleAdd(func() { cm.Toggle() })
		case "toggle-controls", "toggle-notif-center":
			if cm.win.Visible() {
				glib.IdleAdd(func() { cm.win.SetVisible(false) })
			}
		}
	})

	return cm
}

func (cm *CalendarMedia) build(refs *servicerefs.ServiceRefs) {
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

	panel.Append(widgets.BuildMediaGroup(cm.bus, refs.Mpris))
	panel.Append(widgets.BuildCalendarGroup())
	panel.Append(widgets.NewQuickToggles(cm.bus, refs))

	scroll.SetChild(panel)
	root.Append(scroll)
	cm.win.SetChild(root)
	cm.root = root
}

func (cm *CalendarMedia) positionUnderTrigger() {
	monW := surfaceutil.MonitorWidth()
	if monW <= 0 {
		return
	}

	triggerX := surfaceutil.WidgetXRelativeToRoot(cm.trigger)
	triggerW := surfaceutil.WidgetWidth(cm.trigger)
	popupW := panelWidth + panelMargin*2

	desiredLeft := triggerX + triggerW/2 - popupW/2
	if desiredLeft < panelMargin {
		desiredLeft = panelMargin
	}
	if desiredLeft+popupW > monW-panelMargin {
		desiredLeft = monW - panelMargin - popupW
	}

	cm.root.SetMarginTop(layershell.BarExclusiveZone + 8)
	cm.root.SetMarginStart(desiredLeft)
}

func (cm *CalendarMedia) Toggle() {
	if cm.win.Visible() {
		cm.win.SetVisible(false)
	} else {
		cm.positionUnderTrigger()
		cm.win.SetVisible(true)
	}
}

func (cm *CalendarMedia) Close() {
	cm.win.SetVisible(false)
}
