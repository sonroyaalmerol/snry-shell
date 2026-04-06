// Package bar provides the top-edge status bar surface.
package bar

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
)

// Bar is the top-edge status bar surface.
type Bar struct {
	Win            *gtk.ApplicationWindow
	bus            *bus.Bus
	monitor        *gdk.Monitor
	NotifTrigger     gtk.Widgetter
	WifiTrigger      gtk.Widgetter
	BtTrigger        gtk.Widgetter
	ClockGroup       gtk.Widgetter
	TitleTrigger     gtk.Widgetter
	AppDrawerTrigger gtk.Widgetter
}

// New creates and shows the bar window on the given monitor.
func New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs, mon *gdk.Monitor) *Bar {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:         "snry-bar",
		Layer:        layershell.LayerTop,
		Anchors:      layershell.TopEdgeAnchors(),
		KeyboardMode: layershell.KeyboardModeOnDemand,
		Namespace:    "snry-bar",
		Monitor:      mon,
	})

	bar := &Bar{Win: win, bus: b, monitor: mon}
	bar.build(refs)

	// Set exclusive zone from actual allocated height after layout so the
	// compositor reserves exactly the right amount of space regardless of
	// font scaling or CSS changes.
	win.ConnectRealize(func() {
		glib.IdleAdd(func() {
			if h := win.AllocatedHeight(); h > 0 {
				layershell.SetExclusiveZone(win, h)
				layershell.SetBarHeight(h)
			}
		})
	})

	win.SetVisible(true)
	return bar
}

func (b *Bar) build(refs *servicerefs.ServiceRefs) {
	root := gtk.NewCenterBox()
	root.AddCSSClass("bar")
	root.SetStartWidget(b.buildLeft(refs))
	root.SetCenterWidget(b.buildCenter(refs))
	root.SetEndWidget(b.buildRight(refs))
	b.Win.SetChild(root)
}

// Left: app drawer + window title + window mgmt.
func (b *Bar) buildLeft(refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.SetVAlign(gtk.AlignCenter)

	appDrawer := clickableBarGroup(newAppDrawerIcon(), b.bus, "toggle-appdrawer", b.monitor)
	b.AppDrawerTrigger = appDrawer
	box.Append(appDrawer)

	box.Append(newWindowTitleWidget(b.bus, refs.Hyprland, b.monitor))
	b.TitleTrigger = newWindowTitleTrigger
	box.Append(barSeparator())

	return box
}

// Center: workspaces only.
func (b *Bar) buildCenter(refs *servicerefs.ServiceRefs) gtk.Widgetter {
	return barGroup(newWorkspacesWidget(b.bus, refs.Hyprland))
}

func newClockGroup(b *bus.Bus) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.SetVAlign(gtk.AlignCenter)
	box.Append(newClockWidget())
	box.Append(newBatteryIndicator(b))
	return box
}

// Right: individual indicator icons + system tray + clock.
func (b *Bar) buildRight(refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 6)
	box.SetVAlign(gtk.AlignCenter)

	notif := clickableBarGroup(newNotificationIcon(b.bus), b.bus, "toggle-notif-center", b.monitor)
	b.NotifTrigger = notif

	wifi := clickableBarGroup(newWifiIcon(b.bus), b.bus, "toggle-wifi", b.monitor)
	b.WifiTrigger = wifi

	bt := clickableBarGroup(newBluetoothIcon(b.bus, refs), b.bus, "toggle-bluetooth", b.monitor)
	b.BtTrigger = bt

	clockGroup := clickableBarGroup(newClockGroup(b.bus), b.bus, "toggle-calendar", b.monitor)
	b.ClockGroup = clockGroup

	box.Append(notif)
	box.Append(wifi)
	box.Append(bt)
	box.Append(newTrayWidget(b.bus))
	box.Append(barSeparator())
	box.Append(clockGroup)
	return box
}
