// Package controlpanel provides a standalone Material Design 3 control panel
// for managing snry-shell and external tool configurations.
package controlpanel

import (
	"os"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/assets"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
	"github.com/sonroyaalmerol/snry-shell/internal/theme"
)

// ConfigProvider defines the interface for configuration providers
type ConfigProvider interface {
	// Name returns the display name of this provider
	Name() string
	// Icon returns the Material icon name for this provider
	Icon() string
	// Load loads the configuration from persistent storage
	Load() error
	// Save saves the configuration to persistent storage
	Save() error
	// BuildWidget returns the GTK widget for this provider's settings
	BuildWidget() gtk.Widgetter
}

// Run creates and runs the control panel application.
func Run() int {
	app := gtk.NewApplication("sh.snry.shell.controlpanel", 0)

	var win *gtk.ApplicationWindow

	app.ConnectActivate(func() {
		// If a window already exists, just present it.
		if win != nil {
			win.Present()
			return
		}

		// Load embedded stylesheet (same as main shell)
		display := gdk.DisplayGetDefault()
		if display != nil {
			provider := gtk.NewCSSProvider()
			provider.LoadFromString(assets.StyleCSS)
			gtk.StyleContextAddProviderForDisplay(display, provider, gtk.STYLE_PROVIDER_PRIORITY_USER)

			// Load dynamic theme if it exists
			themePath := theme.ThemePath()
			if _, err := os.Stat(themePath); err == nil {
				themeProvider := gtk.NewCSSProvider()
				themeProvider.LoadFromPath(themePath)
				gtk.StyleContextAddProviderForDisplay(display, themeProvider, gtk.STYLE_PROVIDER_PRIORITY_USER+100)
			}
		}

		win = gtk.NewApplicationWindow(app)
		win.SetTitle("Control Panel")
		win.SetDefaultSize(900, 700)
		win.SetResizable(true)

		win.ConnectCloseRequest(func() bool {
			win = nil
			return false
		})

		// Load shell settings
		cfg := settings.DefaultConfig()
		if loaded, err := settings.Load(); err == nil {
			cfg = loaded
		}

		// Build the control panel UI
		cp := newControlPanel(cfg)
		widget := cp.build()
		win.SetChild(widget)

		win.SetVisible(true)
	})

	// Pass only the program name without arguments to avoid GTK parsing --control-panel
	return app.Run([]string{os.Args[0]})
}
