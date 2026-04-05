// Package bar provides the top-edge status bar surface.
package bar

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
)

// Bar is the top-edge status bar surface.
type Bar struct {
	win         *gtk.ApplicationWindow
	bus         *bus.Bus
	StatusGroup gtk.Widgetter
	NotifPill   gtk.Widgetter
	ClockGroup  gtk.Widgetter
}

// New creates and shows the bar window.
func New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs) *Bar {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-bar",
		Layer:         layershell.LayerTop,
		Anchors:       layershell.TopEdgeAnchors(),
		KeyboardMode:  layershell.KeyboardModeOnDemand,
		ExclusiveZone: layershell.BarExclusiveZone,
		Namespace:     "snry-bar",
	})

	bar := &Bar{win: win, bus: b}
	bar.build(refs)
	win.SetVisible(true)
	return bar
}

func (b *Bar) build(refs *servicerefs.ServiceRefs) {
	root := gtk.NewBox(gtk.OrientationHorizontal, 0)
	root.AddCSSClass("bar")

	// Left: window title + status indicators (expands to fill space).
	left := b.buildLeft(refs)
	left.(*gtk.Box).SetHExpand(true)

	// Center: workspaces.
	center := b.buildCenter(refs)

	// Right: pill + tray + clock (natural size).
	right := b.buildRight(refs)

	root.Append(left)
	root.Append(center)
	root.Append(right)
	b.win.SetChild(root)
}

// Left: window title + status indicators.
func (b *Bar) buildLeft(refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.SetVAlign(gtk.AlignCenter)

	statusGroup := clickableBarGroup(newStatusWidgetGroup(b.bus, refs), b.bus, "toggle-controls")
	b.StatusGroup = statusGroup

	box.Append(newWindowTitleWidget(b.bus))
	box.Append(barSeparator())
	box.Append(statusGroup)
	return box
}

// Center: workspaces only.
func (b *Bar) buildCenter(refs *servicerefs.ServiceRefs) gtk.Widgetter {
	ws := barGroup(newWorkspacesWidget(b.bus, refs.Hyprland))
	ws.(*gtk.Box).SetHAlign(gtk.AlignCenter)
	return ws
}

func newClockGroup(b *bus.Bus) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.SetVAlign(gtk.AlignCenter)
	box.Append(newClockWidget())
	return box
}

// Right: indicator pill + system tray + clock.
func (b *Bar) buildRight(refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 6)
	box.SetVAlign(gtk.AlignCenter)

	pillBox := clickableBarGroup(newIndicatorPill(b.bus, refs), b.bus, "toggle-notif-center")
	b.NotifPill = pillBox

	clockGroup := clickableBarGroup(newClockGroup(b.bus), b.bus, "toggle-calendar")
	b.ClockGroup = clockGroup

	box.Append(pillBox)
	box.Append(newTrayWidget(b.bus))
	box.Append(barSeparator())
	box.Append(clockGroup)
	return box
}
