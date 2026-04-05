package notifcenter

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
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
	root    *gtk.Box
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

	panel := gtk.NewBox(gtk.OrientationVertical, 8)
	panel.AddCSSClass("popup-panel")
	panel.SetMarginStart(panelMargin)
	panel.SetMarginEnd(panelMargin)
	panel.SetSizeRequest(panelWidth, -1)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("popup-scroll")
	scroll.SetMaxContentHeight(800)
	scroll.SetPropagateNaturalHeight(true)

	content := gtk.NewBox(gtk.OrientationVertical, 8)
	content.Append(widgets.NewNotificationList(nc.bus))
	content.Append(gtkutil.M3Divider())
	content.Append(widgets.BuildMediaGroup(nc.bus, nc.refs.Mpris))

	scroll.SetChild(content)
	panel.Append(scroll)
	root.Append(panel)

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
		surfaceutil.PositionUnderTrigger(nc.root, nc.trigger, panelWidth, panelMargin)
		nc.win.SetVisible(true)
	}
}
