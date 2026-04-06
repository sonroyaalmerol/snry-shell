package bluetooth

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

// Bluetooth is a popup showing the Bluetooth device picker.
type Bluetooth struct {
	win     *gtk.ApplicationWindow
	bus     *bus.Bus
	refs    *servicerefs.ServiceRefs
	trigger gtk.Widgetter
	monitor *gdk.Monitor
	root    *gtk.Box
}

// New creates and hides the Bluetooth popup anchored to the given trigger widget.
func New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs, trigger gtk.Widgetter) *Bluetooth {
	win, _, root := surfaceutil.NewPopupPanel(app, b, surfaceutil.PopupPanelConfig{
		Name:      "snry-bluetooth",
		Namespace: "snry-bluetooth",
		CloseOn:   []string{"toggle-notif-center", "toggle-wifi", "toggle-calendar", "toggle-overview"},
		Align:     gtk.AlignEnd,
	})

	bt := &Bluetooth{win: win, bus: b, refs: refs, trigger: trigger, root: root}

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

	scroll.SetChild(widgets.NewBluetoothWidget(bt.bus, refs, bt.win))
	panel.Append(scroll)
	root.Append(panel)

	b.Subscribe(bus.TopicPopupTrigger, func(e bus.Event) {
		pt, ok := e.Data.(surfaceutil.PopupTrigger)
		if !ok {
			return
		}
		if pt.Action == "toggle-bluetooth" {
			bt.trigger = pt.Trigger
			bt.monitor = pt.Monitor
		}
	})

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-bluetooth" {
			glib.IdleAdd(func() { bt.Toggle() })
		}
	})

	return bt
}

func (bt *Bluetooth) Toggle() {
	if bt.win.Visible() {
		bt.win.SetVisible(false)
	} else {
		if bt.monitor != nil {
			layershell.SetMonitor(bt.win, bt.monitor)
		}
		surfaceutil.PositionUnderTrigger(bt.root, bt.trigger, panelWidth, panelMargin, bt.monitor)
		bt.win.SetVisible(true)
	}
}
