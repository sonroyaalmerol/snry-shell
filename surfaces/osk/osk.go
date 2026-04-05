// Package osk provides an on-screen keyboard surface.
package osk

import (
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/uinput"
)

type OSK struct {
	win           *gtk.ApplicationWindow
	bus           *bus.Bus
	ui            *uinput.Bridge
	shift         bool
	caps          bool
	ctrlL         bool
	altL          bool
	hasTouch      bool
	manualOff     bool
	visible       bool
	fullscreen    bool
	exclusiveZone int
	keys          []*keyButton          // all character keys, for label updates
	modBtns       map[string]*gtk.Button // modifier name -> button widget
	shiftBtns     []*gtk.Button         // shift buttons for visual feedback
	capsBtn       *gtk.Button           // caps button for visual feedback
	mu            sync.Mutex
	debounce      *time.Timer           // coalesces rapid focus events
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

	// Connect virtual keyboard via /dev/uinput (primary) or ydotoold (fallback).
	ui, err := uinput.New()
	if err != nil {
		log.Printf("[OSK] warning: keyboard input unavailable (%v)", err)
	}
	osk.ui = ui

	osk.build()
	win.SetVisible(false)

	osk.exclusiveZone = 280
	layershell.SetExclusiveZone(win, osk.exclusiveZone)

	osk.hasTouch = detectTouchDevice()

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

	// Auto-show/hide based on zwp_input_method_v2 activate/deactivate events.
	b.Subscribe(bus.TopicTextInputFocus, func(e bus.Event) {
		isText, ok := e.Data.(bool)
		if !ok {
			return
		}
		if isText && osk.manualOff {
			return // user dismissed — don't auto-show for this field
		}
		if !isText {
			osk.manualOff = false // field lost focus, allow auto-show next time
		}
		osk.scheduleFocusUpdate(isText)
	})

	// Drop exclusive zone when a window goes fullscreen so the OSK
	// overlays on top instead of pushing content up.
	b.Subscribe(bus.TopicFullscreen, func(e bus.Event) {
		fs, ok := e.Data.(bool)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			osk.fullscreen = fs
			osk.updateExclusiveZone()
		})
	})

	return osk
}

// scheduleFocusUpdate debounces focus events (300ms) to prevent show/hide
// thrashing from rapid activate/deactivate sequences.
func (o *OSK) scheduleFocusUpdate(want bool) {
	want = o.hasTouch && want
	log.Printf("[OSK] focus: want=%v touch=%v", want, o.hasTouch)

	if o.debounce != nil {
		o.debounce.Stop()
	}
	o.debounce = time.AfterFunc(300*time.Millisecond, func() {
		glib.IdleAdd(func() {
			if want && !o.visible {
				log.Printf("[OSK] auto-showing")
				o.show()
			} else if !want && o.visible {
				log.Printf("[OSK] auto-hiding")
				o.hide()
			}
		})
	})
}

func (o *OSK) show() {
	o.win.SetVisible(true)
	o.win.Present() // raise above all other overlay surfaces
	o.visible = true
}

func (o *OSK) hide() {
	o.win.SetVisible(false)
	o.visible = false
}

func (o *OSK) updateExclusiveZone() {
	if o.fullscreen {
		layershell.SetExclusiveZone(o.win, -1)
	} else {
		layershell.SetExclusiveZone(o.win, o.exclusiveZone)
	}
}

func detectTouchDevice() bool {
	out, err := exec.Command("libinput", "list-devices").Output()
	if err == nil {
		found := strings.Contains(string(out), "touch")
		log.Printf("[OSK] touch detect (libinput): %v", found)
		return found
	}
	log.Printf("[OSK] libinput not available, falling back to hyprctl")
	out, err = exec.Command("hyprctl", "devices", "-j").Output()
	if err != nil {
		log.Printf("[OSK] hyprctl devices also failed: %v", err)
		return false
	}
	raw := string(out)
	idx := strings.Index(raw, `"touch"`)
	if idx < 0 {
		log.Printf("[OSK] touch detect (hyprctl): no touch key in output")
		return false
	}
	rest := raw[idx:]
	after := ""
	if i := strings.Index(rest, ": ["); i >= 0 {
		after = strings.TrimLeft(rest[i+3:], " \t\r\n")
	}
	found := len(after) > 0 && after[0] != ']'
	log.Printf("[OSK] touch detect (hyprctl): %v", found)
	return found
}

type keyDef struct {
	label     string
	normal    string
	shifted   string
	key       string // wtype key name (BackSpace, Tab, Return, Escape, Left, etc.)
	mod       string
	class     string
	action    string
	repeatKey bool
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
	closeBtn.AddCSSClass("osk-key-close")
	closeLabel := gtk.NewLabel("close")
	closeLabel.AddCSSClass("osk-key-label")
	closeLabel.AddCSSClass("material-icon")
	closeBtn.SetChild(closeLabel)
	closeGesture := gtk.NewGestureClick()
	closeGesture.ConnectPressed(func(int, float64, float64) {
		closeGesture.SetState(gtk.EventSequenceClaimed)
		o.manualOff = true
		o.hide()
	})
	closeBtn.AddController(closeGesture)
	topRow.Append(closeBtn)
	root.Append(topRow)

	numRow := []keyDef{
		{label: "Esc", key: "Escape"},
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
		{label: "⌫", key: "BackSpace", class: "osk-key-wide"},
	}

	row1 := []keyDef{
		{label: "Tab", key: "Tab", class: "osk-key-wide"},
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
		{label: "Caps", action: "caps", class: "osk-key-wide"},
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
		{label: "⏎", key: "Return", class: "osk-key-wide"},
	}

	row3 := []keyDef{
		{label: "⇧", action: "shift", class: "osk-key-wide"},
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
		{label: "⇧", action: "shift", class: "osk-key-wide"},
	}

	row4 := []keyDef{
		{label: "Ctrl", mod: "Ctrl_L", class: "osk-key-wide"},
		{label: "Alt", mod: "Alt_L", class: "osk-key-wide"},
		{label: "", normal: " ", class: "osk-key-space"},
		{label: "←", key: "Left", class: "osk-key-arrow"},
		{label: "↓", key: "Down", class: "osk-key-arrow"},
		{label: "↑", key: "Up", class: "osk-key-arrow"},
		{label: "→", key: "Right", class: "osk-key-arrow"},
	}

	for _, row := range [][]keyDef{numRow, row1, row2, row3, row4} {
		o.buildRow(root, row)
	}

	o.win.SetChild(root)
	o.updateKeyLabels()
}

func (o *OSK) buildRow(parent *gtk.Box, defs []keyDef) {
	box := gtk.NewBox(gtk.OrientationHorizontal, 3)
	box.AddCSSClass("osk-row")
	box.SetHExpand(true)
	box.SetHAlign(gtk.AlignCenter)

	for _, d := range defs {
		btn := gtk.NewButton()
		btn.AddCSSClass("osk-key")
		if d.class != "" {
			btn.AddCSSClass(d.class)
		}
		label := gtk.NewLabel(d.label)
		label.AddCSSClass("osk-key-label")
		btn.SetChild(label)

		// Use GestureClick on capture phase so the key fires immediately
		// on touch-down. ConnectClicked waits for full gesture recognition
		// (press + release) which intermittently drops touch events.
		gesture := gtk.NewGestureClick()
		gesture.ConnectPressed(func(int, float64, float64) {
			gesture.SetState(gtk.EventSequenceClaimed)
		})

		switch {
		case d.action == "shift":
			o.shiftBtns = append(o.shiftBtns, btn)
			gesture.ConnectReleased(func(int, float64, float64) {
				o.toggleShift()
			})
		case d.action == "caps":
			o.capsBtn = btn
			gesture.ConnectReleased(func(int, float64, float64) {
				o.toggleCaps()
			})
		case d.mod != "":
			mod := d.mod
			o.modBtns[mod] = btn
			gesture.ConnectReleased(func(int, float64, float64) {
				o.toggleMod(mod)
			})
		default:
			kb := &keyButton{btn: btn, label: label, normal: d.normal, shifted: d.shifted}
			if d.normal != "" || d.shifted != "" {
				o.keys = append(o.keys, kb)
			}
			gesture.ConnectReleased(func(int, float64, float64) {
				o.typeKey(d, kb)
			})
		}

		btn.AddController(gesture)
		box.Append(btn)
	}

	parent.Append(box)
}

func (o *OSK) typeKey(d keyDef, kb *keyButton) {
	if o.ui == nil {
		return
	}
	o.mu.Lock()

	if d.key != "" {
		// Special key (BackSpace, Tab, Return, arrows, Escape).
		o.ui.TypeKey(d.key, o.ctrlL, o.altL, o.shift)
		o.releaseAllModsLocked()
		o.mu.Unlock()
		o.updateKeyLabels()
		return
	}

	if kb == nil {
		o.mu.Unlock()
		return
	}

	// Regular character — resolve via shift/caps state.
	// Snapshot modifier state before releasing so they're applied to this keystroke.
	ch := o.activeChar(kb)
	ctrl, alt := o.ctrlL, o.altL
	o.releaseAllModsLocked()
	o.mu.Unlock()
	o.updateKeyLabels()

	o.ui.TypeChar(ch, ctrl, alt)
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

func (o *OSK) toggleShift() {
	o.mu.Lock()
	o.shift = !o.shift
	o.updateModVisualsLocked()
	o.mu.Unlock()
	o.updateKeyLabels()
}

func (o *OSK) toggleCaps() {
	o.mu.Lock()
	o.caps = !o.caps
	o.updateModVisualsLocked()
	o.mu.Unlock()
	o.updateKeyLabels()
}

func (o *OSK) toggleMod(mod string) {
	o.mu.Lock()
	switch mod {
	case "Ctrl_L":
		o.ctrlL = !o.ctrlL
	case "Alt_L":
		o.altL = !o.altL
	}
	o.updateModVisualsLocked()
	o.mu.Unlock()
}

func (o *OSK) updateModVisualsLocked() {
	cls := "osk-key-active"
	for _, btn := range o.shiftBtns {
		if o.shift {
			glib.IdleAdd(func() { btn.AddCSSClass(cls) })
		} else {
			glib.IdleAdd(func() { btn.RemoveCSSClass(cls) })
		}
	}
	if o.capsBtn != nil {
		if o.caps {
			glib.IdleAdd(func() { o.capsBtn.AddCSSClass(cls) })
		} else {
			glib.IdleAdd(func() { o.capsBtn.RemoveCSSClass(cls) })
		}
	}
	for _, btn := range o.modBtns {
		if btn != nil {
			glib.IdleAdd(func() { btn.RemoveCSSClass(cls) })
		}
	}
	if btn, ok := o.modBtns["Ctrl_L"]; ok {
		if o.ctrlL {
			glib.IdleAdd(func() { btn.AddCSSClass(cls) })
		}
	}
	if btn, ok := o.modBtns["Alt_L"]; ok {
		if o.altL {
			glib.IdleAdd(func() { btn.AddCSSClass(cls) })
		}
	}
}

func (o *OSK) releaseAllModsLocked() {
	if !o.shift && !o.ctrlL && !o.altL {
		return
	}
	o.shift = false
	o.ctrlL = false
	o.altL = false
	o.updateModVisualsLocked()
}
