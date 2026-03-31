// Package osk provides an on-screen keyboard surface.
package osk

import (
	"os/exec"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
)

type OSK struct {
	win *gtk.ApplicationWindow
	bus *bus.Bus
}

func New(app *gtk.Application, b *bus.Bus) *OSK {
	win := gtk.NewApplicationWindow(app)
	win.SetDecorated(false)
	win.SetName("snry-osk")

	layershell.InitForWindow(win)
	layershell.SetLayer(win, layershell.LayerOverlay)
	layershell.SetAnchor(win, layershell.EdgeBottom, true)
	layershell.SetAnchor(win, layershell.EdgeLeft, true)
	layershell.SetAnchor(win, layershell.EdgeRight, true)
	layershell.SetKeyboardMode(win, layershell.KeyboardModeNone)
	layershell.SetExclusiveZone(win, -1)
	layershell.SetNamespace(win, "snry-osk")

	osk := &OSK{win: win, bus: b}
	osk.build()
	win.SetVisible(false)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-osk" {
			glib.IdleAdd(func() { win.SetVisible(!win.Visible()) })
		}
	})

	return osk
}

func (o *OSK) build() {
	root := gtk.NewBox(gtk.OrientationVertical, 0)
	root.AddCSSClass("osk")

	rows := [][]string{
		{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0", "⌫", "⌦"},
		{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p", "⌫", "⌦"},
		{"a", "s", "d", "f", "g", "h", "j", "k", "l", ";", "'", "⏎"},
		{"⇧", "z", "x", "c", "v", "b", "n", "m", ",", ".", "/", "⇩"},
		{"⌨", " ", "⏎"},
	}

	for _, row := range rows {
		grid := gtk.NewGrid()
		grid.AddCSSClass("osk-row")
		grid.SetColumnSpacing(2)
		grid.SetRowSpacing(2)
		grid.SetHAlign(gtk.AlignCenter)

		for col, key := range row {
			btn := gtk.NewButton()
			btn.AddCSSClass("osk-key")
			label := gtk.NewLabel(key)
			label.AddCSSClass("osk-key-label")
			btn.SetChild(label)
			btn.SetSizeRequest(40, 40)

			k := key
			btn.ConnectClicked(func() {
				o.typeKey(k)
			})

			grid.Attach(btn, col, 0, 1, 1)
		}
		root.Append(grid)
	}

	o.win.SetChild(root)
}

func (o *OSK) typeKey(key string) {
	go func() {
		_ = exec.Command("wtype", key).Run()
	}()
}
