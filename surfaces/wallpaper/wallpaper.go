// Package wallpaper provides a built-in wallpaper surface using wlr-layer-shell.
// It renders the configured wallpaper at the Background layer so no external
// daemon (swww, swaybg, hyprpaper, wbg) is required.
package wallpaper

import (
	"log"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
	"github.com/sonroyaalmerol/snry-shell/internal/theme"
)

// fitMode maps a settings string to a GTK ContentFit value.
func fitMode(fit string) gtk.ContentFit {
	switch fit {
	case "contain":
		return gtk.ContentFitContain
	case "fill":
		return gtk.ContentFitFill
	case "scale-down":
		return gtk.ContentFitScaleDown
	default: // "cover"
		return gtk.ContentFitCover
	}
}

// Surface is a full-screen background layer-shell window that displays the
// wallpaper image. Create one per connected monitor.
type Surface struct {
	Win     *gtk.ApplicationWindow
	picture *gtk.Picture
	monitor *gdk.Monitor
}

// New creates a wallpaper surface on mon and subscribes it to wallpaper and
// settings changes published on b.
func New(app *gtk.Application, b *bus.Bus, mon *gdk.Monitor) *Surface {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:  "snry-wallpaper",
		Layer: layershell.LayerBackground,
		Anchors: map[layershell.Edge]bool{
			layershell.EdgeTop:    true,
			layershell.EdgeBottom: true,
			layershell.EdgeLeft:   true,
			layershell.EdgeRight:  true,
		},
		KeyboardMode:  layershell.KeyboardModeNone,
		ExclusiveZone: -1,
		Namespace:     "snry-wallpaper",
		Monitor:       mon,
	})

	picture := gtk.NewPicture()
	picture.SetHExpand(true)
	picture.SetVExpand(true)
	picture.AddCSSClass("wallpaper-surface")
	win.SetChild(picture)

	// Apply the current fit mode and initial wallpaper before showing.
	if cfg, err := settings.Load(); err == nil {
		picture.SetContentFit(fitMode(cfg.WallpaperFit))
	} else {
		picture.SetContentFit(gtk.ContentFitCover)
	}

	if last := theme.GetLastWallpaper(); last != "" {
		picture.SetFilename(last)
	}

	s := &Surface{Win: win, picture: picture, monitor: mon}

	// Update wallpaper image when a new one is processed.
	b.Subscribe(bus.TopicThemeChanged, func(e bus.Event) {
		path, ok := e.Data.(string)
		if !ok || path == "" {
			return
		}
		glib.IdleAdd(func() {
			picture.SetFilename(path)
			log.Printf("[WALLPAPER] surface updated to %s", path)
		})
	})

	// Update fit mode when settings change.
	b.Subscribe(bus.TopicSettingsChanged, func(e bus.Event) {
		cfg, ok := e.Data.(settings.Config)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			picture.SetContentFit(fitMode(cfg.WallpaperFit))
		})
	})

	win.SetVisible(true)
	return s
}

// Close destroys the surface window.
func (s *Surface) Close() {
	s.Win.Close()
}
