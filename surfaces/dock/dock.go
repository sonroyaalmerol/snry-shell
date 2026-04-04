// Package dock provides the application dock surface.
package dock

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/launcher"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
)

// Dock is a bottom-edge application dock.
type Dock struct {
	win *gtk.ApplicationWindow
	bus *bus.Bus
}

func New(app *gtk.Application, b *bus.Bus) *Dock {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-dock",
		Layer:         layershell.LayerTop,
		Anchors:       layershell.BottomEdgeAnchors(),
		ExclusiveZone: 64,
		Namespace:     "snry-dock",
	})

	d := &Dock{win: win, bus: b}
	d.build()
	win.SetVisible(true)
	return d
}

func (d *Dock) build() {
	box := gtk.NewBox(gtk.OrientationHorizontal, 8)
	box.AddCSSClass("dock")
	box.SetHAlign(gtk.AlignCenter)

	// Pinned apps — load from XDG desktop files.
	pinned := pinnedApps()
	for _, app := range pinned {
		icon := gtk.NewImage()
		icon.SetFromIconName(app.Icon)
		icon.SetIconSize(gtk.IconSizeLarge)

		btn := gtk.NewButton()
		btn.AddCSSClass("dock-btn")
		btn.SetChild(icon)
		btn.SetTooltipText(app.Name)
		a := app // capture
		btn.ConnectClicked(func() {
			go launcher.Launch(a) //nolint:errcheck
		})
		box.Append(btn)
	}

	d.win.SetChild(box)
}

// pinnedApps returns a hardcoded default set of pinned applications.
// In a full implementation this would be read from a config file.
func pinnedApps() []launcher.App {
	return []launcher.App{
		{Name: "Firefox", Exec: "firefox", Icon: "firefox"},
		{Name: "Files", Exec: "nautilus", Icon: "org.gnome.Nautilus"},
		{Name: "Terminal", Exec: "foot", Icon: "foot"},
		{Name: "VSCode", Exec: "code", Icon: "code"},
	}
}
