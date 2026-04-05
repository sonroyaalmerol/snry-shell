package widgets

import (
	"os/exec"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

const (
	quickToggleCols = 2
	quickToggleRowH = 44
	quickToggleGap  = 8
)

// NewQuickToggles creates the quick toggle grid using ConstraintLayout
// to ensure equal column widths. panelWidth is the available width for
// the grid content (panel width minus margins).
func NewQuickToggles(b *bus.Bus, refs *servicerefs.ServiceRefs, panelWidth int) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.AddCSSClass("quick-toggles")

	label := gtk.NewLabel("Quick Settings")
	label.AddCSSClass("notif-group-header")
	label.SetHAlign(gtk.AlignStart)
	box.Append(label)

	layout := gtk.NewConstraintLayout()
	root := gtk.NewBox(gtk.OrientationHorizontal, 0)
	root.SetLayoutManager(layout)
	root.AddCSSClass("quick-toggles-grid")

	type toggleDef struct {
		icon     string
		label    string
		topic    bus.Topic
		requires string
		button   bool
		toggle   func(active bool)
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
			go func() { _ = exec.Command("hyprctl", "keyword", "decoration:dim_strength", val).Run() }()
		}},
		{icon: "mic", label: "Mic Mute", requires: "wpctl", toggle: func(_ bool) {
			go func() { _ = exec.Command("wpctl", "set-mute", "@DEFAULT_SOURCE@", "toggle").Run() }()
		}},
		{icon: "equalizer", label: "EasyEffects", requires: "easyeffects", toggle: func(_ bool) {
			go func() { _ = exec.Command("easyeffects", "-t").Run() }()
		}},
		{icon: "notifications_off", label: "DND", topic: bus.TopicDND, toggle: func(active bool) {
			b.Publish(bus.TopicDND, active)
		}},
		{icon: "keep_public", label: "Idle Off", requires: "hyprctl", toggle: func(active bool) {
			action := "close"
			if active {
				action = "open"
			}
			go func() { _ = exec.Command("hyprctl", "dispatch", "inhibit-activity", action).Run() }()
		}},
		{icon: "sports_esports", label: "GameMode", requires: "gamemoderectl", toggle: func(_ bool) {
			go func() { _ = exec.Command("gamemoderectl", "-t").Run() }()
		}},
		{icon: "speed", label: "Performance", requires: "powerprofilesctl", toggle: func(active bool) {
			profile := "balanced"
			if active {
				profile = "performance"
			}
			go func() { _ = exec.Command("powerprofilesctl", "set", profile).Run() }()
		}},
		{icon: "screenshot", label: "Screenshot", button: true, toggle: func(_ bool) {
			b.Publish(bus.TopicSystemControls, "toggle-region-selector")
		}},
		{icon: "colorize", label: "Color Pick", requires: "hyprpicker", button: true, toggle: func(_ bool) {
			go func() { _ = exec.Command("hyprpicker").Run() }()
		}},
		{icon: "keyboard", label: "On-Screen Keyboard", button: true, toggle: func(_ bool) {
			b.Publish(bus.TopicSystemControls, "toggle-osk")
		}},
	}

	type btnEntry struct {
		btn gtk.Widgetter
		def toggleDef
		isPB bool
	}
	var btns []btnEntry

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

		if toggle.button {
			btn := gtk.NewButton()
			btn.SetCursorFromName("pointer")
			btn.AddCSSClass("quick-toggle")
			btn.AddCSSClass("quick-toggle-button")
			btn.SetChild(inner)
			btn.ConnectClicked(func() {
				toggle.toggle(true)
			})
			btns = append(btns, btnEntry{btn: btn, def: toggle, isPB: true})
		} else {
			tb := gtk.NewToggleButton()
			tb.SetCursorFromName("pointer")
			tb.AddCSSClass("quick-toggle")
			tb.SetChild(inner)

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

			btns = append(btns, btnEntry{btn: tb, def: toggle, isPB: false})
		}
	}

	// Add buttons as children and place them into the constraint layout.
	strength := int(gtk.ConstraintStrengthRequired)
	colWidth := float64(panelWidth-quickToggleGap*(quickToggleCols-1)) / float64(quickToggleCols)

	var firstTarget gtk.ConstraintTargetter
	for i, entry := range btns {
		root.Append(entry.btn)

		w := surfaceutil.AsWidget(entry.btn)
		target := &w.ConstraintTarget
		col := i % quickToggleCols
		row := i / quickToggleCols

		layout.AddConstraint(gtk.NewConstraintConstant(
			target, gtk.ConstraintAttributeWidth, gtk.ConstraintRelationEq,
			colWidth, strength))

		layout.AddConstraint(gtk.NewConstraintConstant(
			target, gtk.ConstraintAttributeLeft, gtk.ConstraintRelationEq,
			float64(col)*(colWidth+float64(quickToggleGap)), strength))

		layout.AddConstraint(gtk.NewConstraintConstant(
			target, gtk.ConstraintAttributeTop, gtk.ConstraintRelationEq,
			float64(row)*(float64(quickToggleRowH)+float64(quickToggleGap)), strength))

		layout.AddConstraint(gtk.NewConstraintConstant(
			target, gtk.ConstraintAttributeHeight, gtk.ConstraintRelationEq,
			float64(quickToggleRowH), strength))

		if firstTarget == nil {
			firstTarget = target
		} else {
			layout.AddConstraint(gtk.NewConstraint(
				target, gtk.ConstraintAttributeWidth, gtk.ConstraintRelationEq,
				firstTarget, gtk.ConstraintAttributeWidth,
				1.0, 0.0, strength))
		}
	}

	// Set the overall grid height so the parent sizes correctly.
	numRows := (len(btns) + quickToggleCols - 1) / quickToggleCols
	totalH := numRows*quickToggleRowH + (numRows-1)*quickToggleGap
	gridH := totalH
	if gridH < 0 {
		gridH = 0
	}
	root.SetSizeRequest(panelWidth, gridH)

	box.Append(root)
	return box
}
