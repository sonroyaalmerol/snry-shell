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
	win         *gtk.ApplicationWindow
	bus         *bus.Bus
	shift       bool
	caps        bool
	hasTouch    bool
	manualOff   bool // user explicitly toggled off
	visible     bool
}

func New(app *gtk.Application, b *bus.Bus) *OSK {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-osk",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.BottomEdgeAnchors(),
		KeyboardMode:  layershell.KeyboardModeNone,
		ExclusiveZone: layershell.BarExclusiveZone,
		Namespace:     "snry-osk",
	})

	osk := &OSK{win: win, bus: b}
	osk.build()
	win.SetVisible(false)

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

// detectTouchDevice checks if a touch input device exists via libinput list.
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
	normal  string
	shifted string
	width   int    // column span (0 = default 1)
	class   string // extra CSS class
	action  func(o *OSK)
}

func (o *OSK) build() {
	root := gtk.NewBox(gtk.OrientationVertical, 0)
	root.AddCSSClass("osk")

	numRow := []keyDef{
		{label: "Esc", action: (*OSK).typeEscape},
		{label: "`", normal: "`", shifted: "~"},
		{label: "1", normal: "1", shifted: "!"},
		{label: "2", normal: "2", shifted: "@"},
		{label: "3", normal: "3", shifted: "#"},
		{label: "4", normal: "4", shifted: "$"},
		{label: "5", normal: "5", shifted: "%"},
		{label: "6", normal: "6", shifted: "^"},
		{label: "7", normal: "7", shifted: "&"},
		{label: "8", normal: "8", shifted: "*"},
		{label: "9", normal: "9", shifted: "("},
		{label: "0", normal: "0", shifted: ")"},
		{label: "-", normal: "-", shifted: "_"},
		{label: "=", normal: "=", shifted: "+"},
		{label: "⌫", class: "osk-key-wide", action: (*OSK).typeBackspace},
	}

	row1 := []keyDef{
		{label: "Tab", class: "osk-key-wide", action: (*OSK).typeTab},
		{label: "q", normal: "q", shifted: "Q"},
		{label: "w", normal: "w", shifted: "W"},
		{label: "e", normal: "e", shifted: "E"},
		{label: "r", normal: "r", shifted: "R"},
		{label: "t", normal: "t", shifted: "T"},
		{label: "y", normal: "y", shifted: "Y"},
		{label: "u", normal: "u", shifted: "U"},
		{label: "i", normal: "i", shifted: "I"},
		{label: "o", normal: "o", shifted: "O"},
		{label: "p", normal: "p", shifted: "P"},
		{label: "[", normal: "[", shifted: "{"},
		{label: "]" , normal: "]" , shifted: "}"},
		{label: "\\", normal: "\\", shifted: "|"},
	}

	row2 := []keyDef{
		{label: "Caps", class: "osk-key-wide", action: (*OSK).toggleCaps},
		{label: "a", normal: "a", shifted: "A"},
		{label: "s", normal: "s", shifted: "S"},
		{label: "d", normal: "d", shifted: "D"},
		{label: "f", normal: "f", shifted: "F"},
		{label: "g", normal: "g", shifted: "G"},
		{label: "h", normal: "h", shifted: "H"},
		{label: "j", normal: "j", shifted: "J"},
		{label: "k", normal: "k", shifted: "K"},
		{label: "l", normal: "l", shifted: "L"},
		{label: ";", normal: ";", shifted: ":"},
		{label: "'", normal: "'", shifted: "\""},
		{label: "⏎", class: "osk-key-wide", action: (*OSK).typeEnter},
	}

	row3 := []keyDef{
		{label: "⇧", class: "osk-key-wide", action: (*OSK).toggleShift},
		{label: "z", normal: "z", shifted: "Z"},
		{label: "x", normal: "x", shifted: "X"},
		{label: "c", normal: "c", shifted: "C"},
		{label: "v", normal: "v", shifted: "V"},
		{label: "b", normal: "b", shifted: "B"},
		{label: "n", normal: "n", shifted: "N"},
		{label: "m", normal: "m", shifted: "M"},
		{label: ",", normal: ",", shifted: "<"},
		{label: ".", normal: ".", shifted: ">"},
		{label: "/", normal: "/", shifted: "?"},
		{label: "⇧", class: "osk-key-wide", action: (*OSK).toggleShift},
	}

	row4 := []keyDef{
		{label: "Ctrl", class: "osk-key-wide", action: (*OSK).typeCtrl},
		{label: "Alt", class: "osk-key-wide", action: (*OSK).typeAlt},
		{label: "", normal: " ", class: "osk-key-space"},
		{label: "Alt", class: "osk-key-wide", action: (*OSK).typeAlt},
		{label: "Ctrl", class: "osk-key-wide", action: (*OSK).typeCtrl},
	}

	for _, row := range [][]keyDef{numRow, row1, row2, row3, row4} {
		o.buildRow(root, row)
	}

	o.win.SetChild(root)
}

func (o *OSK) buildRow(parent *gtk.Box, defs []keyDef) {
	box := gtk.NewBox(gtk.OrientationHorizontal, 3)
	box.AddCSSClass("osk-row")
	box.SetHExpand(true)
	box.SetHAlign(gtk.AlignFill)

	for _, k := range defs {
		btn := gtk.NewButton()
		btn.SetCursorFromName("pointer")
		btn.AddCSSClass("osk-key")
		if k.class != "" {
			btn.AddCSSClass(k.class)
		}
		label := gtk.NewLabel(k.label)
		label.AddCSSClass("osk-key-label")
		btn.SetChild(label)

		btn.ConnectClicked(func() {
			if k.action != nil {
				k.action(o)
			} else if k.normal != "" {
				o.typeChar(k)
			}
		})

		box.Append(btn)
	}

	parent.Append(box)
}

func (o *OSK) activeChar(k keyDef) string {
	if o.shift || o.caps {
		if k.shifted != "" {
			return k.shifted
		}
		// For letters, uppercase when shift or caps
		if len(k.normal) == 1 && k.normal[0] >= 'a' && k.normal[0] <= 'z' {
			return string(k.normal[0] - 32)
		}
		return k.normal
	}
	return k.normal
}

func (o *OSK) typeChar(k keyDef) {
	go func() {
		_ = exec.Command("wtype", o.activeChar(k)).Run()
	}()
}

func (o *OSK) typeBackspace() {
	go func() {
		_ = exec.Command("wtype", "-B").Run()
	}()
}

func (o *OSK) typeTab() {
	go func() {
		_ = exec.Command("wtype", "-Tab").Run()
	}()
}

func (o *OSK) typeEnter() {
	go func() {
		_ = exec.Command("wtype", "-Return").Run()
	}()
}

func (o *OSK) typeEscape() {
	go func() {
		_ = exec.Command("wtype", "-Escape").Run()
	}()
}

func (o *OSK) typeCtrl() {
	go func() {
		_ = exec.Command("wtype", "-Ctrl_L").Run()
	}()
}

func (o *OSK) typeAlt() {
	go func() {
		_ = exec.Command("wtype", "-Alt_L").Run()
	}()
}

func (o *OSK) toggleShift() {
	o.shift = !o.shift
}

func (o *OSK) toggleCaps() {
	o.caps = !o.caps
}
