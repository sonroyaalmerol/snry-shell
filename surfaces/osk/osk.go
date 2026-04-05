// Package osk provides an on-screen keyboard surface.
package osk

import (
	"log"
	"os/exec"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
)

type OSK struct {
	win       *gtk.ApplicationWindow
	bus       *bus.Bus
	shift     bool
	caps      bool
	ctrlL     bool
	altL      bool
	hasTouch  bool
	manualOff bool
	visible   bool
	keys      []*keyButton // all character keys, for label updates
	modBtns   map[string]*gtk.Button // modifier name -> button widget
}

type keyButton struct {
	btn     *gtk.Button
	label   *gtk.Label
	normal  string
	shifted string
}

func New(app *gtk.Application, b *bus.Bus) *OSK {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:         "snry-osk",
		Layer:        layershell.LayerOverlay,
		Anchors:      layershell.BottomEdgeAnchors(),
		KeyboardMode: layershell.KeyboardModeNone,
		Namespace:    "snry-osk",
	})

	osk := &OSK{win: win, bus: b, modBtns: make(map[string]*gtk.Button)}
	osk.build()
	win.SetVisible(false)

	// Set a generous exclusive zone so the OSK does not overlap windows.
	layershell.SetExclusiveZone(win, 280)

	osk.hasTouch = detectTouchDevice()

	// Manual toggle via quick settings or bus event.
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-osk" {
			glib.IdleAdd(func() {
				if osk.visible {
					osk.manualOff = true
					osk.hide()
				} else {
					osk.manualOff = false
					osk.show()
				}
			})
		}
	})

	// Auto-show on active window change when touch device is present.
	if osk.hasTouch {
		b.Subscribe(bus.TopicActiveWindow, func(e bus.Event) {
			glib.IdleAdd(func() {
				if !osk.manualOff && !osk.visible {
					osk.show()
				}
			})
		})
		log.Printf("[OSK] touch device detected, auto-trigger enabled")
	}

	return osk
}

func (o *OSK) show() {
	o.win.SetVisible(true)
	o.visible = true
}

func (o *OSK) hide() {
	o.win.SetVisible(false)
	o.visible = false
}

// detectTouchDevice checks if a touch input device exists.
func detectTouchDevice() bool {
	out, err := exec.Command("libinput", "list-devices").Output()
	if err != nil {
		out, err = exec.Command("hyprctl", "devices", "-j").Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), "touch")
	}
	return strings.Contains(string(out), "touch")
}

type keyDef struct {
	label   string
	normal  string  // character to type when not shifted
	shifted string  // character to type when shifted
	key     string  // wtype key name (e.g. BackSpace, Tab, Return, space)
	mod     string  // modifier to hold (e.g. Ctrl_L, Alt_L)
	class   string  // extra CSS class
	action  func(*OSK) // custom action (e.g. toggleShift, toggleCaps)
}

func (o *OSK) build() {
	root := gtk.NewBox(gtk.OrientationVertical, 0)
	root.AddCSSClass("osk")
	root.SetHAlign(gtk.AlignFill)
	root.SetVAlign(gtk.AlignEnd)
	root.SetHExpand(true)

	// Top row: spacer on left, close button on right.
	topRow := gtk.NewBox(gtk.OrientationHorizontal, 0)
	topRow.AddCSSClass("osk-row")
	topRow.SetHExpand(true)
	topRow.SetHAlign(gtk.AlignFill)

	spacer := gtk.NewBox(gtk.OrientationHorizontal, 0)
	spacer.SetHExpand(true)
	topRow.Append(spacer)

	closeBtn := gtk.NewButton()
	closeBtn.SetCursorFromName("pointer")
	closeBtn.AddCSSClass("osk-key-close")
	closeLabel := gtk.NewLabel("close")
	closeLabel.AddCSSClass("osk-key-label")
	closeLabel.AddCSSClass("material-icon")
	closeBtn.SetChild(closeLabel)
	closeBtn.ConnectClicked(func() {
		o.manualOff = true
		o.hide()
	})
	topRow.Append(closeBtn)

	root.Append(topRow)

	numRow := []keyDef{
		{label: "Esc"},
		{normal: "`", shifted: "~"},
		{normal: "1", shifted: "!"},
		{normal: "2", shifted: "@"},
		{normal: "3", shifted: "#"},
		{normal: "4", shifted: "$"},
		{normal: "5", shifted: "%"},
		{normal: "6", shifted: "^"},
		{normal: "7", shifted: "&"},
		{normal: "8", shifted: "*"},
		{normal: "9", shifted: "("},
		{normal: "0", shifted: ")"},
		{normal: "-", shifted: "_"},
		{normal: "=", shifted: "+"},
		{label: "⌫", class: "osk-key-wide", key: "BackSpace"},
	}

	row1 := []keyDef{
		{label: "Tab", class: "osk-key-wide", key: "Tab"},
		{normal: "q", shifted: "Q"},
		{normal: "w", shifted: "W"},
		{normal: "e", shifted: "E"},
		{normal: "r", shifted: "R"},
		{normal: "t", shifted: "T"},
		{normal: "y", shifted: "Y"},
		{normal: "u", shifted: "U"},
		{normal: "i", shifted: "I"},
		{normal: "o", shifted: "O"},
		{normal: "p", shifted: "P"},
		{normal: "[", shifted: "{"},
		{normal: "]", shifted: "}"},
		{normal: "\\", shifted: "|"},
	}

	row2 := []keyDef{
		{label: "Caps", class: "osk-key-wide", action: (*OSK).toggleCaps},
		{normal: "a", shifted: "A"},
		{normal: "s", shifted: "S"},
		{normal: "d", shifted: "D"},
		{normal: "f", shifted: "F"},
		{normal: "g", shifted: "G"},
		{normal: "h", shifted: "H"},
		{normal: "j", shifted: "J"},
		{normal: "k", shifted: "K"},
		{normal: "l", shifted: "L"},
		{normal: ";", shifted: ":"},
		{normal: "'", shifted: "\""},
		{label: "⏎", class: "osk-key-wide", key: "Return"},
	}

	row3 := []keyDef{
		{label: "⇧", class: "osk-key-wide", action: (*OSK).toggleShift},
		{normal: "z", shifted: "Z"},
		{normal: "x", shifted: "X"},
		{normal: "c", shifted: "C"},
		{normal: "v", shifted: "V"},
		{normal: "b", shifted: "B"},
		{normal: "n", shifted: "N"},
		{normal: "m", shifted: "M"},
		{normal: ",", shifted: "<"},
		{normal: ".", shifted: ">"},
		{normal: "/", shifted: "?"},
		{label: "⇧", class: "osk-key-wide", action: (*OSK).toggleShift},
	}

	row4 := []keyDef{
		{label: "Ctrl", class: "osk-key-wide", mod: "Ctrl_L"},
		{label: "Alt", class: "osk-key-wide", mod: "Alt_L"},
		{label: "", normal: " ", class: "osk-key-space", key: "space"},
		{label: "←", class: "osk-key-wide", key: "Left"},
		{label: "↓", class: "osk-key-wide", key: "Down"},
		{label: "↑", class: "osk-key-wide", key: "Up"},
		{label: "→", class: "osk-key-wide", key: "Right"},
		{label: "Alt", class: "osk-key-wide", mod: "Alt_L"},
		{label: "Ctrl", class: "osk-key-wide", mod: "Ctrl_L"},
	}

	for _, row := range [][]keyDef{numRow, row1, row2, row3, row4} {
		o.buildRow(root, row)
	}

	o.win.SetChild(root)

	// Initialize all character key labels with their normal values.
	o.updateKeyLabels()
}

func (o *OSK) buildRow(parent *gtk.Box, defs []keyDef) {
	box := gtk.NewBox(gtk.OrientationHorizontal, 3)
	box.AddCSSClass("osk-row")
	box.SetHExpand(true)
	box.SetHAlign(gtk.AlignCenter)

	for _, d := range defs {
		btn := gtk.NewButton()
		btn.SetCursorFromName("pointer")
		btn.AddCSSClass("osk-key")
		if d.class != "" {
			btn.AddCSSClass(d.class)
		}
		label := gtk.NewLabel(d.label)
		label.AddCSSClass("osk-key-label")
		btn.SetChild(label)

		if d.action != nil {
			btn.ConnectClicked(func() { d.action(o) })
		} else if d.mod != "" {
			mod := d.mod
			o.modBtns[mod] = btn
			btn.ConnectClicked(func() {
				o.toggleMod(mod)
			})
		} else if d.key != "" {
			kb := &keyButton{btn: btn, label: label, normal: d.normal, shifted: d.shifted}
			if d.normal != "" || d.shifted != "" {
				o.keys = append(o.keys, kb)
			}
			btn.ConnectClicked(func() {
				o.typeKey(d, kb)
			})
		} else {
			kb := &keyButton{btn: btn, label: label, normal: d.normal, shifted: d.shifted}
			o.keys = append(o.keys, kb)
			btn.ConnectClicked(func() {
				o.typeKey(d, kb)
			})
		}

		box.Append(btn)
	}

	parent.Append(box)
}

func (o *OSK) typeKey(d keyDef, kb *keyButton) {
	// Build wtype args: held modifiers first, then the key, then release modifiers.
	args := o.modArgs()
	if d.key != "" {
		args = append(args, "-k", d.key)
	} else {
		args = append(args, o.activeChar(kb))
	}
	// Release held modifiers after typing.
	if o.ctrlL {
		args = append(args, "-m", "Ctrl_L")
	}
	if o.altL {
		args = append(args, "-m", "Alt_L")
	}
	if o.shift {
		args = append(args, "-m", "Shift_L")
	}

	go func() {
		_ = exec.Command("wtype", args...).Run()
	}()

	// Release all held modifiers after pressing a non-modifier key.
	glib.IdleAdd(func() {
		o.releaseAllMods()
	})

	// Shift auto-releases after typing a shifted character (Android behavior).
	if d.key == "" && o.shift {
		ch := o.activeChar(kb)
		if ch != kb.normal {
			return // releaseAllMods already handles it
		}
	}
}

// modArgs returns the wtype modifier-hold flags for currently active modifiers.
func (o *OSK) modArgs() []string {
	var args []string
	if o.ctrlL {
		args = append(args, "-M", "Ctrl_L")
	}
	if o.altL {
		args = append(args, "-M", "Alt_L")
	}
	if o.shift {
		args = append(args, "-M", "Shift_L")
	}
	return args
}

// toggleMod toggles a modifier key's held state and updates its visual.
func (o *OSK) toggleMod(mod string) {
	switch mod {
	case "Ctrl_L":
		o.ctrlL = !o.ctrlL
	case "Alt_L":
		o.altL = !o.altL
	}
	if btn, ok := o.modBtns[mod]; ok {
		active := o.ctrlL && mod == "Ctrl_L" || o.altL && mod == "Alt_L"
		if active {
			glib.IdleAdd(func() { btn.AddCSSClass("osk-key-active") })
		} else {
			glib.IdleAdd(func() { btn.RemoveCSSClass("osk-key-active") })
		}
	}
}

// releaseAllMods releases all held modifier keys and updates their visuals.
func (o *OSK) releaseAllMods() {
	o.releaseShift()
	for _, btn := range o.modBtns {
		glib.IdleAdd(func() { btn.RemoveCSSClass("osk-key-active") })
	}
	o.ctrlL = false
	o.altL = false
}

func (o *OSK) activeChar(kb *keyButton) string {
	shifted := o.shift != o.caps
	ch := kb.normal
	if shifted && kb.shifted != "" {
		ch = kb.shifted
	}
	if len(ch) == 1 {
		c := ch[0]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			if shifted {
				ch = strings.ToUpper(ch)
			}
		}
	}
	return ch
}

// updateKeyLabels refreshes all character key labels based on shift/caps state.
func (o *OSK) updateKeyLabels() {
	shifted := o.shift != o.caps
	for _, k := range o.keys {
		ch := k.normal
		if shifted && k.shifted != "" {
			ch = k.shifted
		}
		if len(ch) == 1 {
			c := ch[0]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				if shifted {
					ch = strings.ToUpper(ch)
				}
			}
		}
		glib.IdleAdd(func() { k.label.SetText(ch) })
	}
}

// releaseShift turns off shift (called after typing a shifted char).
func (o *OSK) releaseShift() {
	if o.shift {
		o.shift = false
		o.updateKeyLabels()
	}
}

func (o *OSK) toggleShift() {
	o.shift = !o.shift
	o.updateKeyLabels()
}

func (o *OSK) toggleCaps() {
	o.caps = !o.caps
	o.updateKeyLabels()
}
