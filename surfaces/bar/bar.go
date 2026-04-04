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
	win *gtk.ApplicationWindow
	bus *bus.Bus
}

// New creates and shows the bar window.
func New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs) *Bar {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-bar",
		Layer:         layershell.LayerTop,
		Anchors:       layershell.TopEdgeAnchors(),
		KeyboardMode:  layershell.KeyboardModeOnDemand,
		ExclusiveZone: 40,
		Namespace:     "snry-bar",
	})

	bar := &Bar{win: win, bus: b}
	bar.build(refs)
	win.SetVisible(true)
	return bar
}

func (b *Bar) build(refs *servicerefs.ServiceRefs) {
	root := gtk.NewCenterBox()
	root.AddCSSClass("bar")
	root.SetStartWidget(b.buildLeft())
	root.SetCenterWidget(b.buildCenter(refs))
	root.SetEndWidget(b.buildRight(refs))
	b.win.SetChild(root)
}

// Left: active window title (fills remaining space).
func (b *Bar) buildLeft() gtk.Widgetter {
	return newWindowTitleWidget(b.bus)
}

// Center: [Resources+Media group] | [Workspaces group] | [Clock+Status group].
func (b *Bar) buildCenter(refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.SetVAlign(gtk.AlignCenter)

	// Status indicators group (resources, volume, brightness, battery, keyboard).
	statusGroup := clickableBarGroup(newStatusWidgetGroup(b.bus, refs), b.bus)

	// Workspaces group.
	wsGroup := clickableBarGroup(newWorkspacesWidget(b.bus, refs.Hyprland), b.bus)

	// Clock + media group.
	clockGroup := clickableBarGroup(newClockGroup(b.bus), b.bus)

	box.Append(statusGroup)
	box.Append(barSeparator())
	box.Append(wsGroup)
	box.Append(barSeparator())
	box.Append(clockGroup)

	return box
}

func newClockGroup(b *bus.Bus) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.SetVAlign(gtk.AlignCenter)
	box.Append(newMediaWidget(b))
	box.Append(newClockWidget())
	return box
}

// Right: indicator pill + system tray.
func (b *Bar) buildRight(refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 6)
	box.SetVAlign(gtk.AlignCenter)
	box.Append(newIndicatorPill(b.bus, refs))
	box.Append(newTrayWidget(b.bus))
	return box
}
