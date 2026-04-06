package wifi

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
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
	panelWidth  = 360
)

// WiFi is a popup showing the WiFi network picker.
type WiFi struct {
	win     *gtk.ApplicationWindow
	bus     *bus.Bus
	refs    *servicerefs.ServiceRefs
	trigger gtk.Widgetter
	monitor *gdk.Monitor
	root    *gtk.Box
}

// New creates and hides the WiFi popup anchored to the given trigger widget.
func New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs, trigger gtk.Widgetter) *WiFi {
	win, _, root := surfaceutil.NewPopupPanel(app, b, surfaceutil.PopupPanelConfig{
		Name:      "snry-wifi",
		Namespace: "snry-wifi",
		CloseOn:   []string{"toggle-notif-center", "toggle-bluetooth", "toggle-calendar", "toggle-overview"},
		Align:     gtk.AlignEnd,
	})

	w := &WiFi{win: win, bus: b, refs: refs, trigger: trigger, root: root}

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

	scroll.SetChild(widgets.NewWiFiWidget(w.bus, refs, w.win))
	panel.Append(scroll)
	root.Append(panel)

	b.Subscribe(bus.TopicPopupTrigger, func(e bus.Event) {
		pt, ok := e.Data.(surfaceutil.PopupTrigger)
		if !ok {
			return
		}
		if pt.Action == "toggle-wifi" {
			w.trigger = pt.Trigger
			w.monitor = pt.Monitor
		}
	})

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-wifi" {
			glib.IdleAdd(func() { w.Toggle() })
		}
	})

	return w
}

func (w *WiFi) Toggle() {
	if w.win.Visible() {
		w.win.SetVisible(false)
	} else {
		if w.monitor != nil {
			layershell.SetMonitor(w.win, w.monitor)
		}
		surfaceutil.PositionUnderTrigger(w.root, w.trigger, panelWidth, panelMargin, w.monitor)
		if w.refs.Network != nil {
			go w.refs.Network.ScanWiFi()
		}
		w.win.SetVisible(true)
	}
}
