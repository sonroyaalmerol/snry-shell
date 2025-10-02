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
	win := gtk.NewApplicationWindow(app)
	win.SetDecorated(false)
	win.SetName("snry-bar")

	layershell.InitForWindow(win)
	layershell.SetLayer(win, layershell.LayerTop)
	layershell.SetAnchor(win, layershell.EdgeTop, true)
	layershell.SetAnchor(win, layershell.EdgeLeft, true)
	layershell.SetAnchor(win, layershell.EdgeRight, true)
	layershell.SetExclusiveZone(win, 36)
	layershell.SetNamespace(win, "snry-bar")

	bar := &Bar{win: win, bus: b}
	bar.build(refs)
	win.SetVisible(true)
	return bar
}

func (b *Bar) build(refs *servicerefs.ServiceRefs) {
	root := gtk.NewCenterBox()
	root.AddCSSClass("bar")
	root.SetStartWidget(b.buildLeft())
	root.SetCenterWidget(b.buildCenter())
	root.SetEndWidget(b.buildRight(refs))
	b.win.SetChild(root)
}

func (b *Bar) buildLeft() gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.SetVAlign(gtk.AlignCenter)
	box.Append(newWorkspacesWidget(b.bus))
	box.Append(newWindowTitleWidget(b.bus))
	return box
}

func (b *Bar) buildCenter() gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.SetVAlign(gtk.AlignCenter)
	box.Append(newTrayWidget(b.bus))
	return box
}

func (b *Bar) buildRight(refs *servicerefs.ServiceRefs) gtk.Widgetter {
	return newIndicatorsWidget(b.bus, refs)
}
