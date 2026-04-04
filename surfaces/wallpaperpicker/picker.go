// Package wallpaperpicker provides a wallpaper browser and selection surface.
package wallpaperpicker

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

// WallpaperPicker is a full-screen overlay for browsing and selecting wallpapers.
type WallpaperPicker struct {
	win *gtk.ApplicationWindow
	bus *bus.Bus
}

func New(app *gtk.Application, b *bus.Bus) *WallpaperPicker {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:         "snry-wallpaper-picker",
		Layer:        layershell.LayerOverlay,
		Anchors:      layershell.FullscreenAnchors(),
		KeyboardMode: layershell.KeyboardModeExclusive,
		Namespace:    "snry-wallpaper-picker",
	})

	root := gtk.NewBox(gtk.OrientationVertical, 0)
	root.AddCSSClass("wallpaper-picker")
	win.SetChild(root)

	header := gtk.NewLabel("Wallpapers")
	header.AddCSSClass("wallpaper-picker-header")
	root.Append(header)

	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)

	flow := gtk.NewFlowBox()
	flow.AddCSSClass("wallpaper-grid")
	scroll.SetChild(flow)
	root.Append(scroll)

	wp := &WallpaperPicker{win: win, bus: b}

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "open-wallpaper-picker" || e.Data == "toggle-wallpaper-picker" {
			if win.Visible() {
				win.SetVisible(false)
			} else {
				wp.load(flow)
				win.SetVisible(true)
				win.GrabFocus()
			}
		}
	})

	// Escape to close.
	surfaceutil.AddEscapeToClose(win)
	win.SetVisible(false)
	return wp
}

func (w *WallpaperPicker) load(flow *gtk.FlowBox) {
	dirs := []string{
		os.ExpandEnv("$HOME/Pictures/Wallpapers"),
		os.ExpandEnv("$HOME/.local/share/backgrounds"),
	}
	go func() {
		var paths []string
		for _, dir := range dirs {
			matches, _ := filepath.Glob(filepath.Join(dir, "*.{jpg,jpeg,png,webp}"))
			paths = append(paths, matches...)
		}
		glib.IdleAdd(func() {
			for _, p := range paths {
				btn := gtk.NewButton()
				btn.AddCSSClass("wallpaper-thumb")

				pic := gtk.NewPicture()
				pic.SetFilename(p)
				pic.SetContentFit(gtk.ContentFitCover)
				pic.SetCanShrink(true)
				btn.SetChild(pic)
				btn.SetTooltipText(filepath.Base(p))

				path := p
				btn.ConnectClicked(func() {
					go func() {
						if err := exec.Command("swww", "img", path).Run(); err != nil {
							log.Printf("wallpaper: %v", err)
						}
					}()
				})
				flow.Append(btn)
			}
		})
	}()
}
