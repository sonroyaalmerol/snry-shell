package surfaces

import (
	"context"
	"fmt"
	"os"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/audio"
	"github.com/sonroyaalmerol/snry-shell/internal/services/audiomixer"
	"github.com/sonroyaalmerol/snry-shell/internal/services/bluetooth"
	"github.com/sonroyaalmerol/snry-shell/internal/services/brightness"
	serviceclipboard "github.com/sonroyaalmerol/snry-shell/internal/services/clipboard"
	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
	"github.com/sonroyaalmerol/snry-shell/internal/services/mpris"
	"github.com/sonroyaalmerol/snry-shell/internal/services/network"
	"github.com/sonroyaalmerol/snry-shell/internal/services/nightmode"
	"github.com/sonroyaalmerol/snry-shell/internal/services/notifications"
	"github.com/sonroyaalmerol/snry-shell/internal/services/pomodoro"
	"github.com/sonroyaalmerol/snry-shell/internal/services/resources"
	"github.com/sonroyaalmerol/snry-shell/internal/services/sni"
	"github.com/sonroyaalmerol/snry-shell/internal/services/todo"
	"github.com/sonroyaalmerol/snry-shell/internal/services/upower"
	"github.com/sonroyaalmerol/snry-shell/internal/services/wallpaper"
	"github.com/sonroyaalmerol/snry-shell/internal/controlsocket"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/assets"
	"github.com/sonroyaalmerol/snry-shell/surfaces/bar"
	"github.com/sonroyaalmerol/snry-shell/surfaces/cheatsheet"
	"github.com/sonroyaalmerol/snry-shell/surfaces/clipboard"
	"github.com/sonroyaalmerol/snry-shell/surfaces/corners"
	"github.com/sonroyaalmerol/snry-shell/surfaces/crosshair"
	"github.com/sonroyaalmerol/snry-shell/surfaces/dock"
	"github.com/sonroyaalmerol/snry-shell/surfaces/emoji"
	"github.com/sonroyaalmerol/snry-shell/surfaces/fpslimiter"
	"github.com/sonroyaalmerol/snry-shell/surfaces/imageviewer"
	"github.com/sonroyaalmerol/snry-shell/surfaces/lockscreen"
	"github.com/sonroyaalmerol/snry-shell/surfaces/mediaoverlay"
	"github.com/sonroyaalmerol/snry-shell/surfaces/notes"
	"github.com/sonroyaalmerol/snry-shell/surfaces/notifpopup"
	"github.com/sonroyaalmerol/snry-shell/surfaces/osd"
	"github.com/sonroyaalmerol/snry-shell/surfaces/osk"
	"github.com/sonroyaalmerol/snry-shell/surfaces/overview"
	"github.com/sonroyaalmerol/snry-shell/surfaces/polkit"
	"github.com/sonroyaalmerol/snry-shell/surfaces/recorder"
	"github.com/sonroyaalmerol/snry-shell/surfaces/regionselector"
	"github.com/sonroyaalmerol/snry-shell/surfaces/session"
	"github.com/sonroyaalmerol/snry-shell/surfaces/settings"
	"github.com/sonroyaalmerol/snry-shell/surfaces/sidebar"
	"github.com/sonroyaalmerol/snry-shell/surfaces/wallpaperpicker"
)

// Run creates the GTK application, initialises all services, wires every
// surface and enters the main loop.
func Run() int {
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

	refs := &servicerefs.ServiceRefs{
		Audio:      audio.New(audio.NewRunner(), b),
		Brightness: brightness.New(brightness.NewRunner(), b),
		Mpris:      mpris.New(sysConn, b),
		Bluetooth:  bluetooth.New(sysConn, b),
		Network:    network.New(sysConn, b),
		NightMode:  nightmode.New(nightmode.NewRunner(), nightmode.NewKiller(), b),
		Resources:  resources.New(resources.NewFileReader(), b),
		AudioMixer: audiomixer.New(audiomixer.NewRunner(), b),
		Hyprland:   hyprland.NewQuerier(hyprland.NewCommander()),
		Pomodoro:   pomodoro.New(b),
		Todo:       todo.New(b),
		SNI:        sni.New(sesConn, b),
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
	go refs.SNI.Run()

	// Notification daemon.
	if sesConn != nil {
		notifications.Register(sesConn, notifications.New(b))
	}

	// Clipboard and wallpaper watchers.
	go serviceclipboard.New(serviceclipboard.NewRunner(), b).Run(ctx)
	go wallpaper.New(b).Run(ctx)

	// Hyprland event stream.
	go hyprland.New(hyprland.NewSocketReader(nil), b).Run(ctx)

	// UPower battery monitoring.
	if sysConn != nil {
		go upower.New(sysConn, b).Run(ctx)
	}

	// Subscribe to tray item activation.
	b.Subscribe(bus.TopicTrayActivate, func(ev bus.Event) {
		if id, ok := ev.Data.(string); ok {
			refs.SNI.Activate(id)
		}
	})

	app.ConnectActivate(func() {
		// Load embedded stylesheet.
		display := gdk.DisplayGetDefault()
		if display != nil {
			provider := gtk.NewCSSProvider()
			provider.LoadFromString(assets.StyleCSS)
			gtk.StyleContextAddProviderForDisplay(display, provider, gtk.STYLE_PROVIDER_PRIORITY_USER)
		}

		bar.New(app, b, refs)
		overview.New(app, b, refs.Hyprland)
		sidebar.NewRight(app, b, refs)
		dock.New(app, b)
		osd.New(app, b)
		session.New(app, b)
		corners.New(app, b)
		crosshair.New(app, b)
		lockscreen.New(app, b)
		mediaoverlay.New(app, b, refs.Mpris)
		notifpopup.New(app, b)
		osk.New(app, b)
		regionselector.New(app, b)
		cheatsheet.New(app, b)
		wallpaperpicker.New(app, b)
		settings.New(app, b)
		clipboard.New(app, b)
		emoji.New(app, b)
		notes.New(app, b)
		recorder.New(app, b)
		fpslimiter.New(app, b)
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
