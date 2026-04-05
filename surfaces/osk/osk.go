// Package osk provides an on-screen keyboard surface.
package osk

import (
	"log"
	"os/exec"
	"strings"
	"sync"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// textInputClasses lists window classes (case-insensitive substring match)
// where text input is likely. Terminals, browsers, editors, and chat apps.
var textInputClasses = []string{
	// Terminals
	"kitty", "alacritty", "wezterm", "foot", "ghostty",
	"tilix", "terminator", "konsole", "gnome-terminal",
	"weston-terminal", "xfce4-terminal", "lxterminal",
	// Browsers
	"firefox", "chromium", "chrome", "brave", "vivaldi",
	"edge", "thorium", "zen",
	// Editors / IDEs
	"code", "code-oss", "codium", "cursor",
	"sublime_text", "nvim-qt", "gedit", "mousepad",
	"neovide", "zeditor",
	// Chat / comms
	"telegram", "discord", "vesktop", "element", "nheko",
	"signal", "whatsapp", "skype", "teams",
	// Other text-heavy apps
	"obsidian", "logseq", "joplin", "notion",
	"thunderbird", "geary", "evolution",
	"spotify", "firefox-esr",
}

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
	keys      []*keyButton        // all character keys, for label updates
	modBtns   map[string]*gtk.Button // modifier name -> button widget
	shiftBtns []*gtk.Button        // shift buttons for visual feedback
	capsBtn   *gtk.Button          // caps button for visual feedback
	mu        sync.Mutex
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

	layershell.SetExclusiveZone(win, 280)

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

	// Auto-show/hide based on active window class heuristic.
	b.Subscribe(bus.TopicActiveWindow, func(e bus.Event) {
		if osk.manualOff {
			return
		}
		win, ok := e.Data.(state.ActiveWindow)
		if !ok {
			return
		}
		want := osk.hasTouch && isTextInputWindow(win.Class)
		glib.IdleAdd(func() {
			if want && !osk.visible {
				osk.show()
			} else if !want && osk.visible {
				osk.hide()
			}
		})
	})

	if osk.hasTouch {
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

// isTextInputWindow returns true if the window class looks like an app that
// commonly needs text input. Uses case-insensitive substring matching against
// a known list of terminals, browsers, editors, and chat apps.
func isTextInputWindow(class string) bool {
	lower := strings.ToLower(class)
	for _, match := range textInputClasses {
		if strings.Contains(lower, match) {
			return true
		}
	}
	return false
}

// keyDef describes a single key on the on-screen keyboard.
type keyDef struct {
	label     string // display label
	normal    string // character to type when not shifted
	shifted   string // character to type when shifted
	key       string // wtype key name (BackSpace, Tab, Return, space, Escape, Left, etc.)
	mod       string // modifier to hold (Ctrl_L, Alt_L)
	class     string // extra CSS class
	action    string // "shift", "caps", or "" for none
	repeatKey bool   // true for keys that should auto-repeat on hold (backspace, arrows)
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
		{label: "⌫", key: "BackSpace", class: "osk-key-wide", repeatKey: true},
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
		{label: "", normal: " ", class: "osk-key-space", key: "space"},
		{label: "←", key: "Left", class: "osk-key-arrow", repeatKey: true},
		{label: "↓", key: "Down", class: "osk-key-arrow", repeatKey: true},
		{label: "↑", key: "Up", class: "osk-key-arrow", repeatKey: true},
		{label: "→", key: "Right", class: "osk-key-arrow", repeatKey: true},
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
		btn.SetCursorFromName("pointer")
		btn.AddCSSClass("osk-key")
		if d.class != "" {
			btn.AddCSSClass(d.class)
		}
		label := gtk.NewLabel(d.label)
		label.AddCSSClass("osk-key-label")
		btn.SetChild(label)

		switch {
		case d.action == "shift":
			o.shiftBtns = append(o.shiftBtns, btn)
			btn.ConnectClicked(func() { o.toggleShift() })
		case d.action == "caps":
			o.capsBtn = btn
			btn.ConnectClicked(func() { o.toggleCaps() })
		case d.mod != "":
			mod := d.mod
			o.modBtns[mod] = btn
			btn.ConnectClicked(func() { o.toggleMod(mod) })
		case d.repeatKey:
			o.setupRepeatKey(btn, d)
		default:
			// Regular typable key.
			kb := &keyButton{btn: btn, label: label, normal: d.normal, shifted: d.shifted}
			if d.normal != "" || d.shifted != "" {
				o.keys = append(o.keys, kb)
			}
			o.setupKey(btn, d, kb)
		}

		box.Append(btn)
	}

	parent.Append(box)
}

// setupKey wires a regular (non-repeating) key with press/release visual feedback.
func (o *OSK) setupKey(btn *gtk.Button, d keyDef, kb *keyButton) {
	gesture := gtk.NewGestureClick()
	gesture.SetButton(1)
	gesture.SetPropagationLimit(gtk.LimitNone)
	gesture.ConnectPressed(func(int, float64, float64) {
		btn.AddCSSClass("osk-key-pressed")
		o.typeKey(d, kb)
	})
	gesture.ConnectReleased(func(int, float64, float64) {
		btn.RemoveCSSClass("osk-key-pressed")
	})
	btn.AddController(gesture)
}

// setupRepeatKey wires a key that auto-repeats while held (backspace, arrows).
func (o *OSK) setupRepeatKey(btn *gtk.Button, d keyDef) {
	var cancelled bool

	gesture := gtk.NewGestureClick()
	gesture.SetButton(1)
	gesture.SetPropagationLimit(gtk.LimitNone)
	gesture.ConnectPressed(func(int, float64, float64) {
		btn.AddCSSClass("osk-key-pressed")
		cancelled = false
		o.typeKey(d, nil)
		// Start repeat after initial delay (400ms).
		glib.TimeoutAdd(400, func() bool {
			if cancelled {
				return false
			}
			o.typeKey(d, nil)
			// Continue repeating at faster interval (60ms).
			glib.TimeoutAdd(60, func() bool {
				if cancelled {
					return false
				}
				o.typeKey(d, nil)
				return true
			})
			return false
		})
	})
	gesture.ConnectReleased(func(int, float64, float64) {
		btn.RemoveCSSClass("osk-key-pressed")
		cancelled = true
	})
	btn.AddController(gesture)
}

func (o *OSK) typeKey(d keyDef, kb *keyButton) {
	o.mu.Lock()
	defer o.mu.Unlock()

	args := o.modArgs()
	if d.key != "" {
		args = append(args, "-k", d.key)
	} else if kb != nil {
		args = append(args, o.activeChar(kb))
	}

	if len(args) == 0 {
		return
	}

	// Run synchronously — wtype is fast (sub-ms) and sequential execution
	// avoids race conditions with modifier state.
	_ = exec.Command("wtype", args...).Run()

	// After typing a non-modifier key, release all held modifiers.
	o.releaseAllModsLocked()
}

// modArgs builds the wtype modifier-hold flags for currently active modifiers.
func (o *OSK) modArgs() []string {
	var args []string
	if o.ctrlL {
		args = append(args, "-M", "ctrl")
	}
	if o.altL {
		args = append(args, "-M", "alt")
	}
	if o.shift {
		args = append(args, "-M", "shift")
	}
	return args
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

// updateModVisualsLocked updates the CSS class on all modifier/action buttons.
// Caller must hold o.mu.
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

// releaseAllModsLocked releases all held modifiers and updates visuals.
// Caller must hold o.mu.
func (o *OSK) releaseAllModsLocked() {
	if !o.shift && !o.ctrlL && !o.altL {
		return
	}
	o.shift = false
	o.ctrlL = false
	o.altL = false
	o.updateModVisualsLocked()
}
