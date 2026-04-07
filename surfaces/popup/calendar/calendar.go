package calendar

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
	panelWidth  = 400
)

// Calendar is a popup showing quick toggles and a navigable month calendar.
type Calendar struct {
	win     *gtk.ApplicationWindow
	bus     *bus.Bus
	trigger gtk.Widgetter
	monitor *gdk.Monitor
	root    *gtk.Box
	scroll  *gtk.ScrolledWindow
}

// New creates and hides the calendar popup anchored to the given trigger widget.
func New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs, trigger gtk.Widgetter) *Calendar {
	win, _, root := surfaceutil.NewPopupPanel(app, b, surfaceutil.PopupPanelConfig{
		Name:      "snry-calendar",
		Namespace: "snry-calendar",
		CloseOn:   []string{"toggle-notif-center", "toggle-wifi", "toggle-bluetooth", "toggle-overview"},
		Align:     gtk.AlignEnd,
	})

	cal := &Calendar{win: win, bus: b, trigger: trigger, root: root}

	panel := gtk.NewBox(gtk.OrientationVertical, 0)
	panel.AddCSSClass("popup-panel")
	panel.SetMarginStart(panelMargin)
	panel.SetMarginEnd(panelMargin)
	panel.SetSizeRequest(panelWidth, -1)

	// Header
	header := gtk.NewLabel("Calendar")
	header.AddCSSClass("popup-header")
	header.SetHAlign(gtk.AlignStart)
	panel.Append(header)

	cal.scroll = gtk.NewScrolledWindow()
	cal.scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	cal.scroll.AddCSSClass("popup-scroll")
	cal.scroll.SetMaxContentHeight(surfaceutil.PopupMaxHeight(cal.monitor, layershell.BarHeight()))
	cal.scroll.SetPropagateNaturalHeight(true)
	gtkutil.SetupScrollHoverSuppression(cal.scroll)

	content := gtk.NewBox(gtk.OrientationVertical, 8)
	content.Append(widgets.NewQuickToggles(b, refs))
	content.Append(gtkutil.M3Divider())
	content.Append(widgets.BuildCalendarGroup())

	cal.scroll.SetChild(content)
	panel.Append(cal.scroll)
	root.Append(panel)

	b.Subscribe(bus.TopicPopupTrigger, func(e bus.Event) {
		pt, ok := e.Data.(surfaceutil.PopupTrigger)
		if !ok {
			return
		}
		if pt.Action == "toggle-calendar" {
			cal.trigger = pt.Trigger
			cal.monitor = pt.Monitor
		}
	})

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-calendar" {
			glib.IdleAdd(func() { cal.Toggle() })
		}
	})

	return cal
}

func (cal *Calendar) Toggle() {
	if cal.win.Visible() {
		cal.win.SetVisible(false)
	} else {
		if cal.monitor != nil {
			layershell.SetMonitor(cal.win, cal.monitor)
		}
		surfaceutil.PositionUnderTrigger(cal.root, cal.trigger, panelWidth, panelMargin, cal.monitor)
		// Update max height based on current monitor and scroll to top when opening
		if cal.scroll != nil {
			cal.scroll.SetMaxContentHeight(surfaceutil.PopupMaxHeight(cal.monitor, layershell.BarHeight()))
			cal.scroll.SetVAdjustment(gtk.NewAdjustment(0, 0, 0, 0, 0, 0))
		}
		cal.win.SetVisible(true)
	}
}

