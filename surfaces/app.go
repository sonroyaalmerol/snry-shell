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
	"time"

	"github.com/sonroyaalmerol/snry-shell/assets"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/controlsocket"
	"github.com/sonroyaalmerol/snry-shell/internal/inputmethod"
	"github.com/sonroyaalmerol/snry-shell/internal/networkmanager"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/services/audio"
	"github.com/sonroyaalmerol/snry-shell/internal/services/bluetooth"
	"github.com/sonroyaalmerol/snry-shell/internal/services/brightness"
	serviceclipboard "github.com/sonroyaalmerol/snry-shell/internal/services/clipboard"
	"github.com/sonroyaalmerol/snry-shell/internal/services/darkmode"
	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
	"github.com/sonroyaalmerol/snry-shell/internal/services/idle"
	"github.com/sonroyaalmerol/snry-shell/internal/services/inputmode"
	"github.com/sonroyaalmerol/snry-shell/internal/services/mpris"
	"github.com/sonroyaalmerol/snry-shell/internal/services/network"
	"github.com/sonroyaalmerol/snry-shell/internal/services/nightmode"
	"github.com/sonroyaalmerol/snry-shell/internal/services/notifications"
	"github.com/sonroyaalmerol/snry-shell/internal/services/resources"
	"github.com/sonroyaalmerol/snry-shell/internal/services/sni"
	"github.com/sonroyaalmerol/snry-shell/internal/services/upower"
	shellsettings "github.com/sonroyaalmerol/snry-shell/internal/settings"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
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
	popupnetwork "github.com/sonroyaalmerol/snry-shell/surfaces/popup/network"
	"github.com/sonroyaalmerol/snry-shell/surfaces/popup/notifcenter"
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

	sysConn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Printf("[SHELL] system bus: %v", err)
	}
	if sysConn != nil {
		defer sysConn.Close()
	}
	sesConn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Printf("[SHELL] session bus: %v", err)
	}
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
		Hyprland:   hyprland.NewQuerierWithDefaults(),
		SNI:        sni.New(sesConn, b),
		InputMode:  inputmode.New(b, sysConn, cfg, true),
		DarkMode:   darkmode.New(b, cfg),
		SystemHandler: idle.NewSystemHandler(b, sysConn, cfg.LidCloseAction, cfg.PowerButtonAction),
	}

	// Apply initial step settings
	refs.Audio.UpdateStep(cfg.VolumeStep)
	refs.Brightness.UpdateStep(cfg.BrightnessStep)

	// Initialize shared network manager singleton for unified network state
	if sysConn != nil {
		nmManager := networkmanager.GetInstance(sysConn, b)
		_ = nmManager // The manager starts itself and handles all network operations
		log.Printf("[SHELL] Shared NetworkManager initialized")
	}

	// Start background services.
	go refs.Audio.Run(ctx)
	go refs.Brightness.Run(ctx)
	go refs.Mpris.Run(ctx)
	go refs.Bluetooth.Run(ctx)
	go refs.Network.Run(ctx)
	go refs.Resources.Run(ctx)
	go refs.InputMode.Run(ctx)
	go refs.SNI.Run(ctx)
	go refs.DarkMode.Run(ctx)
	if sysConn != nil {
		go refs.SystemHandler.Run(ctx)
	}

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

	// Idle service — replaces hypridle.
	idleCfg := idle.Config{
		LockTimeout:       idleDuration(cfg.IdleLockTimeout),
		DisplayOffTimeout: idleDuration(cfg.IdleDisplayOffTimeout),
		SuspendTimeout:    idleDuration(cfg.IdleSuspendTimeout),
	}
	idleSvc := idle.New(b, sysConn, idleCfg)
	go idleSvc.Run(ctx)

	var bars []*bar.Bar

	// Update configs when settings change.
	b.Subscribe(bus.TopicSettingsChanged, func(e bus.Event) {
		if newCfg, ok := e.Data.(shellsettings.Config); ok {
			idleSvc.UpdateConfig(idle.Config{
				LockTimeout:       idleDuration(newCfg.IdleLockTimeout),
				DisplayOffTimeout: idleDuration(newCfg.IdleDisplayOffTimeout),
				SuspendTimeout:    idleDuration(newCfg.IdleSuspendTimeout),
			})
			refs.SystemHandler.UpdateConfig(newCfg.LidCloseAction, newCfg.PowerButtonAction)
			refs.Audio.UpdateStep(newCfg.VolumeStep)
			refs.Brightness.UpdateStep(newCfg.BrightnessStep)

			// Reload bars if position changed
			if newCfg.BarPosition != cfg.BarPosition {
				glib.IdleAdd(func() {
					for _, b := range bars {
						b.Reload(newCfg.BarPosition)
					}
				})
			}
			cfg = newCfg
		}
	})

	// toggle-lock command from CLI or keybind.
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-lock" {
			b.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: true})
		}
	})

	// System power actions from session menu or CLI — use logind D-Bus via SystemHandler.
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		cmd, ok := e.Data.(string)
		if !ok || refs.SystemHandler == nil {
			return
		}
		switch cmd {
		case "system-suspend":
			refs.SystemHandler.Suspend()
		case "system-reboot":
			refs.SystemHandler.Reboot()
		case "system-poweroff":
			refs.SystemHandler.PowerOff()
		case "system-logout":
			go refs.Hyprland.SetKeyword("dispatch", "exit")
		}
	})

	// Volume / audio controls.
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		cmd, ok := e.Data.(string)
		if !ok {
			return
		}
		switch cmd {
		case "volume-up":
			if err := refs.Audio.AdjustVolume(refs.Audio.VolumeStep()); err != nil {
				log.Printf("[audio] volume-up: %v", err)
			}
		case "volume-down":
			if err := refs.Audio.AdjustVolume(-refs.Audio.VolumeStep()); err != nil {
				log.Printf("[audio] volume-down: %v", err)
			}
		case "volume-mute":
			if err := refs.Audio.ToggleMute(); err != nil {
				log.Printf("[audio] volume-mute: %v", err)
			}
		case "mic-mute":
			if err := refs.Audio.ToggleMicMute(); err != nil {
				log.Printf("[audio] mic-mute: %v", err)
			}
		}
	})

	// Media controls.
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		cmd, ok := e.Data.(string)
		if !ok {
			return
		}
		switch cmd {
		case "media-play-pause":
			if err := refs.Mpris.PlayPauseActive(); err != nil {
				log.Printf("[mpris] play-pause: %v", err)
			}
		case "media-next":
			if err := refs.Mpris.NextActive(); err != nil {
				log.Printf("[mpris] next: %v", err)
			}
		case "media-prev":
			if err := refs.Mpris.PrevActive(); err != nil {
				log.Printf("[mpris] prev: %v", err)
			}
		}
	})

	// Brightness controls.
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		cmd, ok := e.Data.(string)
		if !ok {
			return
		}
		switch cmd {
		case "brightness-up":
			if err := refs.Brightness.AdjustBrightness(refs.Brightness.BrightnessStep()); err != nil {
				log.Printf("[brightness] up: %v", err)
			}
		case "brightness-down":
			if err := refs.Brightness.AdjustBrightness(-refs.Brightness.BrightnessStep()); err != nil {
				log.Printf("[brightness] down: %v", err)
			}
		}
	})

	// Zoom controls — adjust Hyprland cursor:zoom_factor at runtime.
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		cmd, ok := e.Data.(string)
		if !ok {
			return
		}
		switch cmd {
		case "zoom-in":
			adjustZoom(refs.Hyprland, +0.3)
		case "zoom-out":
			adjustZoom(refs.Hyprland, -0.3)
		case "zoom-reset":
			if err := refs.Hyprland.SetKeyword("cursor:zoom_factor", "1"); err != nil {
				log.Printf("[zoom] reset: %v", err)
			}
		}
	})

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
				refs.DarkMode.UpdateConfig(newCfg)
				log.Printf("[SETTINGS] Reloaded settings from control panel")
			}
		}
	})

	// Hyprland event stream.
	if conn, err := net.Dial("unix", hyprland.SocketPath()); err != nil {
		log.Printf("[SHELL] hyprland socket: %v (window events disabled)", err)
	} else {
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

	// Register Hyprland keybindings for power button and lid switch.
	cleanupSystemBinds := setupHyprlandSystemBinds(refs.Hyprland)
	defer cleanupSystemBinds()

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
				bars = append(bars, bar.New(app, b, refs, mon, cfg.BarPosition))
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
		popupnetwork.New(app, b, refs, bars[0].NetworkTrigger)
		popupbluetooth.New(app, b, refs, bars[0].BtTrigger)
		windowmgmt.New(app, b, refs, bars[0].TitleTrigger)
		calendar.New(app, b, refs, bars[0].ClockGroup)
		osd.New(app, b)
		session.New(app, b)
		crosshair.New(app, b)
		ls := lockscreen.New(app, b)
		// Load initial lockscreen settings
		ls.UpdateSettings(cfg)

		b.Subscribe(bus.TopicSettingsChanged, func(e bus.Event) {
			if newCfg, ok := e.Data.(shellsettings.Config); ok {
				ls.UpdateSettings(newCfg)
			}
		})

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
				fmt.Fprintf(os.Stderr, "polkit agent: %v\\n", err)
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

// idleDuration converts an integer seconds value from settings to a
// time.Duration. Zero means disabled (returns 0).
func idleDuration(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// setupHyprlandSystemBinds registers bindl keybindings for power button and lid
// switch so Hyprland forwards those events to the shell via the control socket.
// Returns a cleanup function that removes the bindings; call it on shell exit.
func setupHyprlandSystemBinds(q *hyprland.Querier) func() {
	binds := []struct{ key, cmd string }{
		{"XF86PowerOff", "toggle-power-action"},
		{"switch:on:Lid Switch", "toggle-lid-action"},
	}
	for _, b := range binds {
		val := ", " + b.key + ", exec, snry-shell --" + b.cmd
		if err := q.SetKeyword("bindl", val); err != nil {
			log.Printf("[SHELL] hyprland bindl %s: %v", b.key, err)
		} else {
			log.Printf("[SHELL] hyprland bindl registered: %s -> %s", b.key, b.cmd)
		}
	}
	return func() {
		for _, b := range binds {
			if err := q.SetKeyword("unbind", ", "+b.key); err != nil {
				log.Printf("[SHELL] hyprland unbind %s: %v", b.key, err)
			}
		}
	}
}

// adjustZoom reads the current cursor:zoom_factor, adds delta, clamps to [1, 3],
// and applies it via hyprctl keyword.
func adjustZoom(q *hyprland.Querier, delta float64) {
	raw, err := q.GetOption("cursor:zoom_factor")
	if err != nil {
		log.Printf("[zoom] getoption: %v", err)
		return
	}
	var current float64
	if _, err := fmt.Sscanf(raw, "%f", &current); err != nil {
		current = 1.0
	}
	next := current + delta
	if next < 1.0 {
		next = 1.0
	}
	if next > 3.0 {
		next = 3.0
	}
	if err := q.SetKeyword("cursor:zoom_factor", fmt.Sprintf("%.2f", next)); err != nil {
		log.Printf("[zoom] set: %v", err)
	}
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
