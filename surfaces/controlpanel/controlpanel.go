// Package controlpanel provides a standalone Material Design 3 control panel
// for managing snry-shell and external tool configurations.
package controlpanel

import (
	"os"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
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

	app.ConnectActivate(func() {
		window := gtk.NewApplicationWindow(app)
		window.SetTitle("Control Panel")
		window.SetDefaultSize(900, 700)
		window.SetResizable(true)

		// Load shell settings
		cfg := settings.DefaultConfig()
		if loaded, err := settings.Load(); err == nil {
			cfg = loaded
		}

		// Build the control panel UI
		cp := newControlPanel(cfg)
		window.SetChild(cp.build())

		window.SetVisible(true)
	})

	return app.Run(os.Args)
}
