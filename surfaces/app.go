package surfaces

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/assets"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/controlsocket"
	"github.com/sonroyaalmerol/snry-shell/internal/inputmethod"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/services/audio"
	"github.com/sonroyaalmerol/snry-shell/internal/services/audiomixer"
	"github.com/sonroyaalmerol/snry-shell/internal/services/bluetooth"
	"github.com/sonroyaalmerol/snry-shell/internal/services/brightness"
	serviceclipboard "github.com/sonroyaalmerol/snry-shell/internal/services/clipboard"
	"github.com/sonroyaalmerol/snry-shell/internal/services/darkmode"
	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
	"github.com/sonroyaalmerol/snry-shell/internal/services/inputmode"
	"github.com/sonroyaalmerol/snry-shell/internal/services/mpris"
	"github.com/sonroyaalmerol/snry-shell/internal/services/network"
	"github.com/sonroyaalmerol/snry-shell/internal/services/nightmode"
	"github.com/sonroyaalmerol/snry-shell/internal/services/notifications"
	"github.com/sonroyaalmerol/snry-shell/internal/services/pomodoro"
	"github.com/sonroyaalmerol/snry-shell/internal/services/resources"
	"github.com/sonroyaalmerol/snry-shell/internal/services/sni"
	"github.com/sonroyaalmerol/snry-shell/internal/services/todo"
	"github.com/sonroyaalmerol/snry-shell/internal/services/upower"
	shellsettings "github.com/sonroyaalmerol/snry-shell/internal/settings"
	"github.com/sonroyaalmerol/snry-shell/internal/theme"
	"github.com/sonroyaalmerol/snry-shell/surfaces/bar"
	"github.com/sonroyaalmerol/snry-shell/surfaces/cheatsheet"
	"github.com/sonroyaalmerol/snry-shell/surfaces/corners"
	"github.com/sonroyaalmerol/snry-shell/surfaces/crosshair"
	"github.com/sonroyaalmerol/snry-shell/surfaces/imageviewer"
	"github.com/sonroyaalmerol/snry-shell/surfaces/lockscreen"
	"github.com/sonroyaalmerol/snry-shell/surfaces/mediaoverlay"
	"github.com/sonroyaalmerol/snry-shell/surfaces/notes"
	"github.com/sonroyaalmerol/snry-shell/surfaces/notifpopup"
	"github.com/sonroyaalmerol/snry-shell/surfaces/osd"
	"github.com/sonroyaalmerol/snry-shell/surfaces/osk"
	"github.com/sonroyaalmerol/snry-shell/surfaces/overview"
	"github.com/sonroyaalmerol/snry-shell/surfaces/polkit"
	"github.com/sonroyaalmerol/snry-shell/surfaces/popup/appdrawer"
	popupbluetooth "github.com/sonroyaalmerol/snry-shell/surfaces/popup/bluetooth"
	"github.com/sonroyaalmerol/snry-shell/surfaces/popup/calendar"
	"github.com/sonroyaalmerol/snry-shell/surfaces/popup/notifcenter"
	"github.com/sonroyaalmerol/snry-shell/surfaces/popup/wifi"
	"github.com/sonroyaalmerol/snry-shell/surfaces/popup/windowmgmt"
	"github.com/sonroyaalmerol/snry-shell/surfaces/recorder"
	"github.com/sonroyaalmerol/snry-shell/surfaces/regionselector"
	"github.com/sonroyaalmerol/snry-shell/surfaces/session"
	"github.com/sonroyaalmerol/snry-shell/surfaces/settings"
)

// Run creates the GTK application, initialises all services, wires every
// surface and enters the main loop.
func Run() int {
	log.Println("snry-shell: Run() starting")
	b := bus.New()
	app := gtk.NewApplication("sh.snry.shell", 0)

	sysConn, _ := dbus.ConnectSystemBus()
	if sysConn != nil {
		defer sysConn.Close()
	}
	sesConn, _ := dbus.ConnectSessionBus()
	if sesConn != nil {
		defer sesConn.Close()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load settings from persistent store
	cfg := shellsettings.DefaultConfig()
	if loadedCfg, err := shellsettings.Load(); err == nil {
		cfg = loadedCfg
	}

	refs := &servicerefs.ServiceRefs{
		Audio:      audio.NewWithDefaults(b),
		Brightness: brightness.NewWithDefaults(b),
		Mpris:      mpris.New(sysConn, b),
		Bluetooth:  bluetooth.New(sysConn, b),
		Network:    network.New(sysConn, b),
		NightMode:  nightmode.New(nightmode.NewRunner(), nightmode.NewKiller(), b),
		Resources:  resources.New(resources.NewFileReader(), b),
		AudioMixer: audiomixer.NewWithDefaults(b),
		Hyprland:   hyprland.NewQuerierWithDefaults(),
		Pomodoro:   pomodoro.New(b),
		Todo:       todo.New(b),
		SNI:        sni.New(sesConn, b),
		InputMode:  inputmode.New(b, sysConn, cfg, true),
		DarkMode:   darkmode.New(b, cfg),
	}

	// Start background services.
	go refs.Audio.Run(ctx)
	go refs.Brightness.Run(ctx)
	go refs.Mpris.Run(ctx)
	go refs.Bluetooth.Run(ctx)
	go refs.Network.Run(ctx)
	go refs.Resources.Run(ctx)
	go refs.AudioMixer.Run(ctx)
	go refs.Pomodoro.Run(ctx)
	go refs.InputMode.Run(ctx)
	go refs.SNI.Run(ctx)
	go refs.DarkMode.Run(ctx)

	// Notification daemon.
	if sesConn != nil {
		notifications.Register(sesConn, notifications.New(b))
	}

	// Input method watcher for per-field OSK triggering via zwp_input_method_v2.
	imWatcher, err := inputmethod.New(b)
	if err != nil {
		log.Printf("[SHELL] inputmethod: %v", err)
	}
	if imWatcher != nil {
		go imWatcher.Run(ctx)
	}

	// Clipboard watcher.
	go serviceclipboard.NewWithDefaults(b).Run(ctx)

	// Theme generator and wallpaper monitor.
	themeMonitor := theme.NewMonitor(b)
	go themeMonitor.Run(ctx)

	// Handle theme reload command.
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if action, ok := e.Data.(string); ok && action == "toggle-reload-theme" {
			if err := themeMonitor.ForceUpdate(); err != nil {
				log.Printf("[THEME] Force update failed: %v", err)
			}
		}
	})

	// Handle settings reload from control panel.
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if action, ok := e.Data.(string); ok && action == "reload-settings" {
			if newCfg, err := shellsettings.Load(); err == nil {
				cfg = newCfg
				// Publish settings changed event so components can react
				b.Publish(bus.TopicSettingsChanged, cfg)
				log.Printf("[SETTINGS] Reloaded settings from control panel")
			}
		}
	})

	// Hyprland event stream.
	if conn, err := net.Dial("unix", hyprland.SocketPath()); err == nil {
		svc := hyprland.New(hyprland.NewSocketReader(conn), b)
		if clients, err := refs.Hyprland.Clients(); err == nil {
			svc.SeedClients(clients)
		}
		go svc.Run(ctx)
	}

	// UPower battery monitoring.
	if sysConn != nil {
		go upower.New(sysConn, b).Run(ctx)
	}

	// Force Hyprland config values while shell is alive, restore on exit.
	forced := hyprland.NewForcedConfigs(refs.Hyprland)
	if err := forced.Apply([]hyprland.ForcedConfig{
		{Option: "decoration:rounding", Value: "12"},
	}); err != nil {
		log.Printf("[SHELL] forced config error: %v", err)
	} else {
		log.Printf("[SHELL] forced config: applied decoration:rounding=12")
	}
	defer func() {
		log.Printf("[SHELL] forced config: restoring original values")
		forced.Restore()
	}()
	// Subscribe to tray item activation.
	b.Subscribe(bus.TopicTrayActivate, func(ev bus.Event) {
		if id, ok := ev.Data.(string); ok {
			refs.SNI.Activate(id)
		}
	})

	// Subscribe to theme changes and reload CSS.
	b.Subscribe(bus.TopicThemeChanged, func(ev bus.Event) {
		glib.IdleAdd(func() {
			display := gdk.DisplayGetDefault()
			if display != nil {
				loadThemeCSS(display)
			}
		})
	})

	app.ConnectActivate(func() {
		// Load embedded stylesheet.
		display := gdk.DisplayGetDefault()
		if display != nil {
			provider := gtk.NewCSSProvider()
			provider.LoadFromString(assets.StyleCSS)
			gtk.StyleContextAddProviderForDisplay(display, provider, gtk.STYLE_PROVIDER_PRIORITY_USER)

			// Load dynamic theme if it exists
			loadThemeCSS(display)
		}

		// Per-monitor surfaces: bar and corners.
		var bars []*bar.Bar
		var allCorners []*corners.Corners

		refreshMonitors := func() {
			for _, br := range bars {
				br.Win.Close()
			}
			for _, c := range allCorners {
				c.Close()
			}
			bars = nil
			allCorners = nil

			d := gdk.DisplayGetDefault()
			if d == nil {
				return
			}
			monitors := d.Monitors()
			n := monitors.NItems()
			for i := uint(0); i < n; i++ {
				item := monitors.Item(i)
				if item == nil {
					continue
				}
				mon := &gdk.Monitor{Object: item}
				bars = append(bars, bar.New(app, b, refs, mon))
				allCorners = append(allCorners, corners.New(app, b, mon))
			}
			log.Printf("[SHELL] monitors: %d bars created", len(bars))
		}

		refreshMonitors()

		// Watch for monitor hotplug.
		if display != nil {
			display.Monitors().ConnectItemsChanged(func(_, _, _ uint) {
				glib.IdleAdd(refreshMonitors)
			})
		}

		// Use primary bar triggers as defaults for popups.
		overview.New(app, b, refs.Hyprland)
		appdrawer.New(app, b, bars[0].AppDrawerTrigger)
		notifcenter.New(app, b, refs, bars[0].NotifTrigger)
		wifi.New(app, b, refs, bars[0].WifiTrigger)
		popupbluetooth.New(app, b, refs, bars[0].BtTrigger)
		windowmgmt.New(app, b, refs, bars[0].TitleTrigger)
		calendar.New(app, b, refs, bars[0].ClockGroup)
		osd.New(app, b)
		session.New(app, b)
		crosshair.New(app, b)
		lockscreen.New(app, b)
		mediaoverlay.New(app, b, refs.Mpris)
		notifpopup.New(app, b)
		osk.New(app, b)
		regionselector.New(app, b)
		cheatsheet.New(app, b)
		settings.New(app, b)
		notes.New(app, b)
		recorder.New(app, b)
		imageviewer.New(app, b)
		if sysConn != nil {
			agent := polkit.New(sysConn)
			if err := agent.Register(); err != nil {
				fmt.Fprintf(os.Stderr, "polkit agent: %v\n", err)
			}
		}
	})

	// Control socket for --toggle-* commands from CLI.
	sockLn, err := controlsocket.Start(b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "control socket: %v\n", err)
	}
	defer controlsocket.Close(sockLn)

	return app.Run(os.Args)
}

// loadThemeCSS loads the dynamic theme CSS if it exists
func loadThemeCSS(display *gdk.Display) {
	themePath := theme.ThemePath()
	if _, err := os.Stat(themePath); os.IsNotExist(err) {
		return
	}

	provider := gtk.NewCSSProvider()
	provider.LoadFromPath(themePath)
	// Load with higher priority than base CSS so it overrides fallback colors
	gtk.StyleContextAddProviderForDisplay(display, provider, gtk.STYLE_PROVIDER_PRIORITY_USER+100)
	log.Println("[THEME] Loaded dynamic theme from", themePath)
}
