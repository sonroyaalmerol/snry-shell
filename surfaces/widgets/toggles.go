package widgets

import (
	"log"
	"os/exec"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

func newInputModeControl(b *bus.Bus) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("quick-toggle-segmented")
	box.SetHExpand(true)

	type segment struct {
		icon  string
		label string
		mode  string
		btn   *gtk.ToggleButton
	}
	segments := []segment{
		{icon: "auto_awesome", label: "Auto", mode: "auto"},
		{icon: "touch_app", label: "Tablet", mode: "tablet"},
		{icon: "computer", label: "Desktop", mode: "desktop"},
	}

	setting := false

	// Load saved mode from settings
	savedMode := settings.DefaultConfig().InputMode
	if cfg, err := settings.Load(); err == nil && cfg.InputMode != "" {
		savedMode = cfg.InputMode
	}

	for i := range segments {
		seg := &segments[i]
		btn := gtk.NewToggleButton()
		btn.AddCSSClass("quick-toggle-segment")
		btn.SetCursorFromName("pointer")
		seg.btn = btn

		inner := gtk.NewBox(gtk.OrientationHorizontal, 0)
		icon := gtk.NewLabel(seg.icon)
		icon.AddCSSClass("material-icon")
		icon.AddCSSClass("segment-icon")
		lbl := gtk.NewLabel(seg.label)
		lbl.AddCSSClass("segment-label")
		inner.Append(icon)
		inner.Append(lbl)
		btn.SetChild(inner)
		btn.SetHExpand(true)

		if i > 0 {
			btn.SetGroup(segments[0].btn)
		}

		// Set active based on saved mode
		if seg.mode == savedMode {
			btn.SetActive(true)
		}

		btn.ConnectToggled(func() {
			if setting {
				return
			}
			if btn.Active() {
				b.Publish(bus.TopicSystemControls, "set-input-mode:"+seg.mode)
			}
		})

		box.Append(btn)
	}

	b.Subscribe(bus.TopicInputMode, func(e bus.Event) {
		mode, ok := e.Data.(string)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			setting = true
			for _, seg := range segments {
				if seg.mode == mode {
					seg.btn.SetActive(true)
					break
				}
			}
			setting = false
		})
	})

	return box
}

const quickToggleCols = 2

// NewQuickToggles creates the quick toggle grid with equal-width columns.
func NewQuickToggles(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.AddCSSClass("quick-toggles")

	header := gtk.NewBox(gtk.OrientationHorizontal, 0)
	header.SetHExpand(true)

	label := gtk.NewLabel("Quick Settings")
	label.AddCSSClass("notif-group-header")
	label.SetHAlign(gtk.AlignStart)
	label.SetVAlign(gtk.AlignCenter)
	label.SetHExpand(true)

	powerBtn := gtk.NewButton()
	powerBtn.AddCSSClass("quick-power-btn")
	powerBtn.SetTooltipText("Power menu")
	powerBtn.SetHAlign(gtk.AlignEnd)
	powerBtn.SetVAlign(gtk.AlignCenter)
	powerBtn.SetCursorFromName("pointer")
	powerBtn.SetChild(gtkutil.MaterialIcon("power_settings_new"))
	powerBtn.ConnectClicked(func() {
		b.Publish(bus.TopicSystemControls, "toggle-session")
	})

	header.Append(label)
	header.Append(powerBtn)
	box.Append(header)

	grid := gtk.NewGrid()
	grid.AddCSSClass("quick-toggles-grid")
	grid.SetColumnSpacing(8)
	grid.SetRowSpacing(8)
	grid.SetColumnHomogeneous(true)
	grid.SetHExpand(true)

	type toggleDef struct {
		icon      string
		label     string
		topic     bus.Topic
		requires  string
		button    bool
		segmented bool
		toggle    func(active bool)
	}

	binInPath := func(name string) bool {
		_, err := exec.LookPath(name)
		return err == nil
	}

	toggles := []toggleDef{
		{icon: "wifi", label: "WiFi", topic: bus.TopicNetwork, toggle: func(active bool) {
			if refs.Network != nil {
				go refs.Network.SetWiFi(active)
			}
		}},
		{icon: "bluetooth", label: "Bluetooth", topic: bus.TopicBluetooth, toggle: func(active bool) {
			if refs.Bluetooth != nil {
				go refs.Bluetooth.SetPowered(active)
			}
		}},
		{icon: "nightlight", label: "Night Light", topic: bus.TopicNightMode, requires: "hyprsunset", toggle: func(_ bool) {
			if refs.NightMode != nil {
				refs.NightMode.Toggle()
			}
		}},
		{icon: "visibility", label: "Anti-Flash", requires: "hyprctl", toggle: func(active bool) {
			val := "0"
			if active {
				val = "0.3"
			}
			go func() {
				if err := exec.Command("hyprctl", "keyword", "decoration:dim_strength", val).Run(); err != nil {
					log.Printf("toggle anti-flash: %v", err)
				}
			}()
		}},
		{icon: "mic", label: "Mic Mute", requires: "wpctl", toggle: func(_ bool) {
			go func() {
				if err := exec.Command("wpctl", "set-mute", "@DEFAULT_SOURCE@", "toggle").Run(); err != nil {
					log.Printf("toggle mic-mute: %v", err)
				}
			}()
		}},
		{icon: "equalizer", label: "EasyEffects", requires: "easyeffects", toggle: func(_ bool) {
			go func() {
				if err := exec.Command("easyeffects", "-t").Run(); err != nil {
					log.Printf("toggle easyeffects: %v", err)
				}
			}()
		}},
		{icon: "notifications_off", label: "DND", topic: bus.TopicDND, toggle: func(active bool) {
			b.Publish(bus.TopicDND, active)
		}},
		{icon: "keep_public", label: "Idle Off", requires: "hyprctl", toggle: func(active bool) {
			action := "close"
			if active {
				action = "open"
			}
			go func() {
				if err := exec.Command("hyprctl", "dispatch", "inhibit-activity", action).Run(); err != nil {
					log.Printf("toggle idle-off: %v", err)
				}
			}()
		}},
		{icon: "sports_esports", label: "GameMode", requires: "gamemoderectl", toggle: func(_ bool) {
			go func() {
				if err := exec.Command("gamemoderectl", "-t").Run(); err != nil {
					log.Printf("toggle gamemode: %v", err)
				}
			}()
		}},
		{icon: "speed", label: "Performance", requires: "powerprofilesctl", toggle: func(active bool) {
			profile := "balanced"
			if active {
				profile = "performance"
			}
			go func() {
				if err := exec.Command("powerprofilesctl", "set", profile).Run(); err != nil {
					log.Printf("toggle performance: %v", err)
				}
			}()
		}},
		{icon: "screenshot", label: "Screenshot", button: true, toggle: func(_ bool) {
			b.Publish(bus.TopicSystemControls, "toggle-region-selector")
		}},
		{icon: "colorize", label: "Color Pick", requires: "hyprpicker", button: true, toggle: func(_ bool) {
			go func() {
				if err := exec.Command("hyprpicker").Run(); err != nil {
					log.Printf("color picker: %v", err)
				}
			}()
		}},
		{icon: "inputmode", label: "Input Mode", segmented: true},
	}

	col := 0
	row := 0
	for _, t := range toggles {
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

		if toggle.segmented {
			ctrl := newInputModeControl(b)
			grid.Attach(ctrl, col, row, 2, 1)
			col++ // skip the second column
		} else if toggle.button {
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
			tb := gtk.NewToggleButton()
			tb.SetCursorFromName("pointer")
			tb.AddCSSClass("quick-toggle")
			tb.SetChild(inner)
			tb.SetHExpand(true)

			settingState := false
			tb.ConnectToggled(func() {
				if settingState {
					return
				}
				tb.AddCSSClass("loading")
				glib.TimeoutAdd(uint(3000), func() bool {
					tb.RemoveCSSClass("loading")
					return false
				})
				toggle.toggle(tb.Active())
			})

			if toggle.topic != "" {
				topic := toggle.topic
				b.Subscribe(topic, func(e bus.Event) {
					glib.IdleAdd(func() {
						settingState = true
						switch v := e.Data.(type) {
						case state.NetworkState:
							tb.SetActive(v.WirelessEnabled)
						case state.BluetoothState:
							tb.SetActive(v.Powered)
						case bool:
							tb.SetActive(v)
						}
						settingState = false
						tb.RemoveCSSClass("loading")
					})
				})
			}

			grid.Attach(tb, col, row, 1, 1)
		}

		col++
		if col >= quickToggleCols {
			col = 0
			row++
		}
	}

	box.Append(grid)

	// Brightness slider.
	brightnessScale := gtkutil.M3Slider(0, 1, 0.01)
	brightnessScale.AddCSSClass("quick-slider")

	settingBrightness := false
	brightnessScale.ConnectChangeValue(func(_ gtk.ScrollType, value float64) bool {
		if !settingBrightness {
			refs.Brightness.SetBrightness(value)
		}
		return false
	})

	b.Subscribe(bus.TopicBrightness, func(e bus.Event) {
		bs, ok := e.Data.(state.BrightnessState)
		if !ok || bs.Max == 0 {
			return
		}
		glib.IdleAdd(func() {
			settingBrightness = true
			brightnessScale.SetValue(float64(bs.Current) / float64(bs.Max))
			settingBrightness = false
		})
	})

	sliderGrid := gtk.NewGrid()
	sliderGrid.SetColumnSpacing(8)
	sliderGrid.SetRowSpacing(8)
	sliderGrid.SetHExpand(true)

	// Brightness row (row 0).
	brightnessIcon := gtk.NewLabel("brightness_high")
	brightnessIcon.AddCSSClass("material-icon")
	brightnessIcon.AddCSSClass("quick-slider-icon")
	brightnessIcon.SetVAlign(gtk.AlignCenter)

	brightnessLabel := gtk.NewLabel("Brightness")
	brightnessLabel.AddCSSClass("quick-slider-label")
	brightnessLabel.SetHAlign(gtk.AlignStart)
	brightnessLabel.SetVAlign(gtk.AlignCenter)

	sliderGrid.Attach(brightnessIcon, 0, 0, 1, 1)
	sliderGrid.Attach(brightnessLabel, 1, 0, 1, 1)
	sliderGrid.Attach(brightnessScale, 2, 0, 1, 1)

	// Volume slider.
	volumeScale := gtkutil.M3Slider(0, 1, 0.01)
	volumeScale.AddCSSClass("quick-slider")
	volumeScale.ConnectChangeValue(func(_ gtk.ScrollType, value float64) bool {
		refs.Audio.SetVolume(value)
		return false
	})

	// Volume row (row 1).
	volumeIcon := gtk.NewLabel("volume_up")
	volumeIcon.AddCSSClass("material-icon")
	volumeIcon.AddCSSClass("quick-slider-icon")
	volumeIcon.SetVAlign(gtk.AlignCenter)

	volumeLabel := gtk.NewLabel("Volume")
	volumeLabel.AddCSSClass("quick-slider-label")
	volumeLabel.SetHAlign(gtk.AlignStart)
	volumeLabel.SetVAlign(gtk.AlignCenter)

	sliderGrid.Attach(volumeIcon, 0, 1, 1, 1)
	sliderGrid.Attach(volumeLabel, 1, 1, 1, 1)
	sliderGrid.Attach(volumeScale, 2, 1, 1, 1)

	box.Append(sliderGrid)

	return box
}
