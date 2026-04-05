package widgets

import (
	"log"
	"os/exec"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const quickToggleCols = 2

// NewQuickToggles creates the quick toggle grid.
func NewQuickToggles(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.AddCSSClass("quick-toggles")

	label := gtk.NewLabel("Quick Settings")
	label.AddCSSClass("notif-group-header")
	label.SetHAlign(gtk.AlignStart)
	box.Append(label)

	grid := gtk.NewGrid()
	grid.AddCSSClass("quick-toggles-grid")
	grid.SetColumnSpacing(8)
	grid.SetRowSpacing(8)
	grid.SetHExpand(true)

	type toggleDef struct {
		icon     string
		label    string
		topic    bus.Topic
		requires string // binary name; empty string means no external dep
		button   bool   // if true, render as a regular button (one-shot action)
		toggle   func(active bool)
	}

	// binInPath checks whether a command is available on $PATH.
	binInPath := func(name string) bool {
		_, err := exec.LookPath(name)
		return err == nil
	}

	toggles := []toggleDef{
		// ── Connectivity ──
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
		// ── Display ──
		{
			icon:     "nightlight",
			label:    "Night Light",
			topic:    bus.TopicNightMode,
			requires: "hyprsunset",
			toggle: func(_ bool) {
				if refs.NightMode != nil {
					refs.NightMode.Toggle()
				}
			},
		},
		{
			icon:     "visibility",
			label:    "Anti-Flash",
			requires: "hyprctl",
			toggle: func(active bool) {
				val := "0"
				if active {
					val = "0.3"
				}
				go func() { _ = exec.Command("hyprctl", "keyword", "decoration:dim_strength", val).Run() }()
			},
		},
		// ── Audio ──
		{
			icon:     "mic",
			label:    "Mic Mute",
			requires: "wpctl",
			toggle: func(_ bool) {
				go func() { _ = exec.Command("wpctl", "set-mute", "@DEFAULT_SOURCE@", "toggle").Run() }()
			},
		},
		{
			icon:     "equalizer",
			label:    "EasyEffects",
			requires: "easyeffects",
			toggle: func(_ bool) {
				go func() { _ = exec.Command("easyeffects", "-t").Run() }()
			},
		},
		// ── System ──
		{
			icon:  "notifications_off",
			label: "DND",
			topic: bus.TopicDND,
			toggle: func(active bool) {
				b.Publish(bus.TopicDND, active)
			},
		},
		{
			icon:     "keep_public",
			label:    "Idle Off",
			requires: "hyprctl",
			toggle: func(active bool) {
				action := "close"
				if active {
					action = "open"
				}
				go func() { _ = exec.Command("hyprctl", "dispatch", "inhibit-activity", action).Run() }()
			},
		},
		{
			icon:     "sports_esports",
			label:    "GameMode",
			requires: "gamemoderectl",
			toggle: func(_ bool) {
				go func() { _ = exec.Command("gamemoderectl", "-t").Run() }()
			},
		},
		{
			icon:     "speed",
			label:    "Performance",
			requires: "powerprofilesctl",
			toggle: func(active bool) {
				profile := "balanced"
				if active {
					profile = "performance"
				}
				go func() { _ = exec.Command("powerprofilesctl", "set", profile).Run() }()
			},
		},
		// ── Tools ──
		{
			icon:   "screenshot",
			label:  "Screenshot",
			button: true,
			toggle: func(_ bool) {
				b.Publish(bus.TopicSystemControls, "toggle-region-selector")
			},
		},
		{
			icon:     "colorize",
			label:    "Color Pick",
			requires: "hyprpicker",
			button:   true,
			toggle: func(_ bool) {
				go func() { _ = exec.Command("hyprpicker").Run() }()
			},
		},
		{
			icon:   "keyboard",
			label:  "On-Screen Keyboard",
			button: true,
			toggle: func(_ bool) {
				b.Publish(bus.TopicSystemControls, "toggle-osk")
			},
		},
	}

	col := 0
	row := 0
	for _, t := range toggles {
		// Skip toggles whose external dependency is not installed.
		if t.requires != "" && !binInPath(t.requires) {
			continue
		}
		toggle := t

		inner := gtk.NewBox(gtk.OrientationHorizontal, 8)
		inner.SetHAlign(gtk.AlignFill)

		icon := gtk.NewLabel(toggle.icon)
		icon.AddCSSClass("material-icon")
		icon.AddCSSClass("quick-toggle-icon")

		lbl := gtk.NewLabel(toggle.label)
		lbl.AddCSSClass("quick-toggle-label")
		lbl.SetHAlign(gtk.AlignStart)
		lbl.SetVAlign(gtk.AlignCenter)
		lbl.SetXAlign(0)
		lbl.SetWrap(true)

		inner.Append(icon)
		inner.Append(lbl)

		if toggle.button {
			btn := gtk.NewButton()
			btn.SetCursorFromName("pointer")
			btn.AddCSSClass("quick-toggle")
			btn.AddCSSClass("quick-toggle-button")
			btn.SetChild(inner)
			btn.SetHExpand(true)
			btn.ConnectClicked(func() {
				toggle.toggle(true)
			})
			grid.Attach(btn, col, row, 1, 1)
		} else {
			btn := gtk.NewToggleButton()
			btn.SetCursorFromName("pointer")
			btn.AddCSSClass("quick-toggle")
			btn.SetChild(inner)
			btn.SetHExpand(true)

			settingState := false
			btn.ConnectToggled(func() {
				log.Printf("[TOGGLE] %s ConnectToggled: Active=%v, settingState=%v", toggle.label, btn.Active(), settingState)
				if settingState {
					return
				}
				btn.AddCSSClass("loading")
				glib.TimeoutAdd(uint(3000), func() bool {
					btn.RemoveCSSClass("loading")
					return false
				})
				toggle.toggle(btn.Active())
			})

			if toggle.topic != "" {
				topic := toggle.topic
				b.Subscribe(topic, func(e bus.Event) {
					log.Printf("[TOGGLE] %s bus event: %+v", toggle.label, e.Data)
					glib.IdleAdd(func() {
						log.Printf("[TOGGLE] %s IdleAdd: setting settingState=true, current Active=%v", toggle.label, btn.Active())
						settingState = true
						switch v := e.Data.(type) {
						case state.NetworkState:
							btn.SetActive(v.WirelessEnabled)
						case state.BluetoothState:
							btn.SetActive(v.Powered)
						case bool:
							btn.SetActive(v)
						}
						log.Printf("[TOGGLE] %s IdleAdd: now Active=%v, setting settingState=false", toggle.label, btn.Active())
						settingState = false
						btn.RemoveCSSClass("loading")
					})
				})
			}

			grid.Attach(btn, col, row, 1, 1)
		}

		col++
		if col >= quickToggleCols {
			col = 0
			row++
		}
	}

	box.Append(grid)
	return box
}
