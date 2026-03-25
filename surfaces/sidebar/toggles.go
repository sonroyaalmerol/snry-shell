package sidebar

import (
	"os/exec"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// newQuickToggles creates the Android-style quick toggle grid.
func newQuickToggles(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.AddCSSClass("quick-toggles")

	label := gtk.NewLabel("Quick Settings")
	label.AddCSSClass("notif-group-header")
	label.SetHAlign(gtk.AlignStart)
	box.Append(label)

	grid := gtk.NewFlowBox()
	grid.AddCSSClass("quick-toggles-grid")
	grid.SetColumnSpacing(8)
	grid.SetRowSpacing(8)
	grid.SetMaxChildrenPerLine(4)
	grid.SetSelectionMode(gtk.SelectionNone)

	type toggleDef struct {
		icon   string
		label  string
		topic  bus.Topic
		toggle func(active bool)
	}

	toggles := []toggleDef{
		{
			icon:  "wifi",
			label: "WiFi",
			topic: bus.TopicNetwork,
			toggle: func(active bool) {
				if refs.Network != nil {
					go refs.Network.SetWiFi(active)
				}
			},
		},
		{
			icon:  "bluetooth",
			label: "Bluetooth",
			topic: bus.TopicBluetooth,
			toggle: func(active bool) {
				if refs.Bluetooth != nil {
					go refs.Bluetooth.SetPowered(active)
				}
			},
		},
		{
			icon:  "nightlight",
			label: "Night Light",
			topic: bus.TopicNightMode,
			toggle: func(_ bool) {
				if refs.NightMode != nil {
					refs.NightMode.Toggle()
				}
			},
		},
		{
			icon:  "screenshot",
			label: "Screenshot",
			toggle: func(_ bool) {
				b.Publish(bus.TopicSystemControls, "toggle-region-selector")
			},
		},
		{
			icon:  "mic",
			label: "Mic Mute",
			toggle: func(_ bool) {
				go func() { _ = exec.Command("wpctl", "set-mute", "@DEFAULT_SOURCE@", "toggle").Run() }()
			},
		},
		{
			icon:  "power_settings_new",
			label: "Idle Off",
			toggle: func(_ bool) {
				b.Publish(bus.TopicSystemControls, "toggle-crosshair")
			},
		},
		{
			icon:  "wifi",
			label: "WiFi Networks",
			toggle: func(_ bool) {
				if refs.Network != nil {
					go refs.Network.ScanWiFi()
				}
				b.Publish(bus.TopicSystemControls, "open-wifi-picker")
			},
		},
		{
			icon:  "music_note",
			label: "Volume Mixer",
			toggle: func(_ bool) {
				b.Publish(bus.TopicSystemControls, "open-volume-mixer")
			},
		},
	}

	for _, t := range toggles {
		toggle := t
		btn := gtk.NewToggleButton()
		btn.AddCSSClass("quick-toggle")

		inner := gtk.NewBox(gtk.OrientationVertical, 2)
		inner.SetHAlign(gtk.AlignCenter)

		icon := gtk.NewLabel(toggle.icon)
		icon.AddCSSClass("material-icon")
		icon.AddCSSClass("quick-toggle-icon")

		lbl := gtk.NewLabel(toggle.label)
		lbl.AddCSSClass("quick-toggle-label")

		inner.Append(icon)
		inner.Append(lbl)
		btn.SetChild(inner)

		btn.ConnectToggled(func() {
			toggle.toggle(btn.Active())
		})

		// Track state via subscription if topic is set.
		if toggle.topic != "" {
			topic := toggle.topic
			b.Subscribe(topic, func(e bus.Event) {
				glib.IdleAdd(func() {
					switch v := e.Data.(type) {
					case state.NetworkState:
						btn.SetActive(v.WirelessEnabled)
					case state.BluetoothState:
						btn.SetActive(v.Powered)
					case bool:
						btn.SetActive(v)
					}
				})
			})
		}

		grid.Append(btn)
	}

	box.Append(grid)
	return box
}
