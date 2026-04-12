// Package osk provides an on-screen keyboard surface with integrated
// emoji picker and clipboard history panels.
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
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
	"github.com/sonroyaalmerol/snry-shell/internal/uinput"
)

type OSK struct {
	win            *gtk.ApplicationWindow
	bus            *bus.Bus
	ui             *uinput.Bridge
	hasTouch       bool
	stack          *gtk.Stack
	keys           []*keyButton           // all character keys, for label updates
	modBtns        map[string]*gtk.Button // modifier name -> button widget
	shiftBtns      []*gtk.Button          // shift buttons for visual feedback
	capsBtn        *gtk.Button            // caps button for visual feedback
	emojiBtn       *gtk.Button            // toolbar emoji button
	clipboardBtn   *gtk.Button            // toolbar clipboard button
	clipboardList  *gtk.Box               // clipboard list widget for refresh
	emojiContainer *gtk.Box               // vertical box holding category FlowBoxes
	backBtn        *gtk.Button            // floating back-to-keyboard button

	// mu guards all modifier and state fields below.
	mu         sync.Mutex
	shift      bool
	caps       bool
	ctrlL      bool
	altL       bool
	tabletMode bool   // true when no physical keyboard detected
	manualOff  bool   // true when user explicitly closed OSK
	manualMode bool   // true when OSK was manually opened via bar button
	visible    bool
	fullscreen bool
	viewMode   string // "keyboard", "emoji", "clipboard"
	debounce   *time.Timer
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

	osk := &OSK{win: win, bus: b, modBtns: make(map[string]*gtk.Button), viewMode: "keyboard"}

	// Connect virtual keyboard via /dev/uinput (primary) or ydotoold (fallback).
	ui, err := uinput.New()
	if err != nil {
		log.Printf("[OSK] warning: keyboard input unavailable (%v)", err)
	}
	osk.ui = ui

	osk.build()
	win.SetVisible(false)

	// Keep exclusive zone in sync with actual window height.
	win.ConnectMap(func() {
		glib.IdleAdd(func() { osk.updateExclusiveZone() })
	})
	win.ConnectRealize(func() {
		glib.IdleAdd(func() { osk.updateExclusiveZone() })
	})

	osk.hasTouch = detectTouchDevice()

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		action, _ := e.Data.(string)
		switch action {
		case "toggle-osk":
			glib.IdleAdd(func() {
				osk.mu.Lock()
				defer osk.mu.Unlock()
				if osk.visible {
					osk.manualOff = true
					osk.manualMode = false
					osk.hideLocked()
				} else {
					osk.manualOff = false
					osk.manualMode = false
					osk.switchView("keyboard")
					osk.showLocked()
				}
			})
		case "toggle-osk-bar":
			glib.IdleAdd(func() {
				osk.mu.Lock()
				defer osk.mu.Unlock()
				if osk.visible {
					osk.manualOff = true
					osk.manualMode = false
					osk.hideLocked()
				} else {
					osk.manualOff = false
					osk.manualMode = true
					osk.switchView("keyboard")
					osk.showLocked()
				}
			})
		case "toggle-emoji":
			glib.IdleAdd(func() {
				osk.mu.Lock()
				defer osk.mu.Unlock()
				if osk.visible && osk.viewMode == "emoji" {
					osk.switchView("keyboard")
				} else {
					osk.switchView("emoji")
					osk.showLocked()
				}
			})
		case "toggle-clipboard":
			glib.IdleAdd(func() {
				osk.mu.Lock()
				defer osk.mu.Unlock()
				if osk.visible && osk.viewMode == "clipboard" {
					osk.switchView("keyboard")
				} else {
					osk.switchView("clipboard")
					osk.showLocked()
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
		osk.mu.Lock()
		if isText && osk.manualOff {
			osk.mu.Unlock()
			return
		}
		if !isText {
			osk.manualOff = false
		}
		osk.mu.Unlock()
		osk.scheduleFocusUpdate(isText)
	})

	// Update tablet mode state from the inputmode service (bool).
	b.Subscribe(bus.TopicTabletMode, func(e bus.Event) {
		tablet, ok := e.Data.(bool)
		if !ok {
			return
		}
		osk.mu.Lock()
		osk.tabletMode = tablet
		shouldHide := !osk.tabletMode && osk.visible && !osk.manualOff
		osk.mu.Unlock()
		log.Printf("[OSK] tablet mode: %v", tablet)
		if shouldHide {
			glib.IdleAdd(func() {
				osk.mu.Lock()
				defer osk.mu.Unlock()
				osk.hideLocked()
			})
		}
	})

	// Drop exclusive zone when a window goes fullscreen so the OSK
	// overlays on top instead of pushing content up.
	b.Subscribe(bus.TopicFullscreen, func(e bus.Event) {
		fs, ok := e.Data.(bool)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			osk.mu.Lock()
			osk.fullscreen = fs
			osk.mu.Unlock()
			osk.updateExclusiveZone()
		})
	})

	// Handle screen lock/unlock
	b.Subscribe(bus.TopicScreenLock, func(e bus.Event) {
		if ls, ok := e.Data.(state.LockScreenState); ok {
			glib.IdleAdd(func() {
				osk.mu.Lock()
				defer osk.mu.Unlock()
				if ls.Locked {
					osk.hideLocked()
					log.Printf("[OSK] screen locked")
				} else {
					log.Printf("[OSK] screen unlocked")
				}
			})
		}
	})

	return osk
}

// switchView switches the visible panel. Must be called with mu held
// (except for GTK widget calls which must run on the GTK thread).
func (o *OSK) switchView(mode string) {
	o.viewMode = mode
	o.stack.SetVisibleChildName(mode)
	o.updateExclusiveZone()
	o.updateViewButtons()
	if mode == "clipboard" {
		o.refreshClipboard("")
	}
}

func (o *OSK) updateViewButtons() {
	cls := "osk-key-active"
	if o.emojiBtn != nil {
		if o.viewMode == "emoji" {
			o.emojiBtn.AddCSSClass(cls)
		} else {
			o.emojiBtn.RemoveCSSClass(cls)
		}
	}
	if o.clipboardBtn != nil {
		if o.viewMode == "clipboard" {
			o.clipboardBtn.AddCSSClass(cls)
		} else {
			o.clipboardBtn.RemoveCSSClass(cls)
		}
	}
	if o.backBtn != nil {
		glib.IdleAdd(func() {
			o.mu.Lock()
			vm := o.viewMode
			o.mu.Unlock()
			o.backBtn.SetVisible(vm != "keyboard")
		})
	}
}

func (o *OSK) scheduleFocusUpdate(want bool) {
	o.mu.Lock()
	if o.manualMode {
		o.mu.Unlock()
		return
	}

	want = o.tabletMode && want
	log.Printf("[OSK] focus: want=%v tablet=%v manual=%v", want, o.tabletMode, o.manualMode)

	if o.debounce != nil {
		o.debounce.Stop()
	}
	o.debounce = time.AfterFunc(300*time.Millisecond, func() {
		glib.IdleAdd(func() {
			o.mu.Lock()
			defer o.mu.Unlock()
			if want && !o.visible {
				log.Printf("[OSK] auto-showing")
				o.switchView("keyboard")
				o.showLocked()
			} else if !want && o.visible {
				log.Printf("[OSK] auto-hiding")
				o.hideLocked()
			}
		})
	})
	o.mu.Unlock()
}

// showLocked sets the OSK visible. Must be called with mu held.
// Returns true if visibility changed (caller should publish after releasing mu).
func (o *OSK) showLocked() bool {
	if o.visible {
		return false
	}
	o.win.SetVisible(true)
	o.visible = true
	glib.IdleAdd(func() {
		o.win.PresentWithTime(uint32(glib.GetMonotonicTime() / 1000))
	})
	return true
}

// hideLocked hides the OSK. Must be called with mu held.
// Returns true if visibility changed (caller should publish after releasing mu).
func (o *OSK) hideLocked() bool {
	if !o.visible {
		return false
	}
	o.win.SetVisible(false)
	o.visible = false
	return true
}

func (o *OSK) updateExclusiveZone() {
	o.mu.Lock()
	fs := o.fullscreen
	o.mu.Unlock()
	if fs {
		layershell.SetExclusiveZone(o.win, -1)
		return
	}
	h := o.win.AllocatedHeight()
	if h <= 0 {
		_, h, _, _ = gtk.BaseWidget(o.win).Measure(gtk.OrientationVertical, -1)
	}
	if h <= 0 {
		return
	}
	layershell.SetExclusiveZone(o.win, h)
}

func detectTouchDevice() bool {
	out, err := exec.Command("libinput", "list-devices").Output()
	if err == nil {
		found := false
		// Parse device blocks: look for Capabilities lines containing "touch"
		// but NOT "touchpad" to avoid false positives.
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Capabilities:") && strings.Contains(line, "touch") {
				found = true
				break
			}
		}
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
	if _, after0, ok := strings.Cut(rest, ": ["); ok {
		after = strings.TrimLeft(after0, " \t\r\n")
	}
	found := len(after) > 0 && after[0] != ']'
	log.Printf("[OSK] touch detect (hyprctl): %v", found)
	return found
}

// emojiData holds all emoji organised by category for the picker panel.
var emojiData = []struct {
	name   string
	emojis [][2]string
}{
	{"Smileys", [][2]string{
		{"😀", "grinning"}, {"😃", "smiley"}, {"😄", "smile"}, {"😁", "grin"},
		{"😆", "laughing"}, {"😅", "sweat smile"}, {"🤣", "rolling on floor"}, {"😂", "joy"},
		{"🙂", "slightly smiling"}, {"🙃", "upside down"}, {"😉", "wink"}, {"😊", "blush"},
		{"😇", "innocent"}, {"🥰", "smiling heart"}, {"😍", "heart eyes"}, {"🤩", "star struck"},
		{"😘", "kissing heart"}, {"😗", "kissing"}, {"😚", "relieved"}, {"😙", "kissing closed"},
		{"🥲", "smiling tear"}, {"😋", "yum"}, {"😛", "tongue"}, {"😜", "zany"},
		{"🤪", "zany"}, {"😝", "squint tongue"}, {"🤑", "money mouth"}, {"🤗", "hugging"},
		{"🤭", "hand over mouth"}, {"🤫", "shushing"}, {"🤔", "thinking"}, {"🫡", "salute"},
		{"🤐", "zipper mouth"}, {"🤨", "raised eyebrow"}, {"😐", "neutral"}, {"😑", "expressionless"},
		{"😶", "no mouth"}, {"🫥", "dotted line face"}, {"😏", "smirk"}, {"😒", "unamused"},
		{"🙄", "rolling eyes"}, {"😬", "grimacing"}, {"🤥", "lying"}, {"😌", "relieved"},
		{"😔", "pensive"}, {"😪", "sleepy"}, {"🤤", "drooling"}, {"😴", "sleeping"},
		{"😷", "mask"}, {"🤒", "sick"}, {"🤕", "hurt"}, {"🤢", "nauseated"},
		{"🤮", "vomiting"}, {"🥵", "hot"}, {"🥶", "cold"}, {"🥴", "woozy"},
		{"😵", "dizzy"}, {"🤯", "exploding head"}, {"🤠", "cowboy"}, {"🥳", "partying"},
		{"🥸", "disguised"}, {"😎", "cool"}, {"🤓", "nerd"}, {"🧐", "monocle"},
		{"😕", "confused"}, {"🫤", "diagonal mouth"}, {"😟", "worried"}, {"🙁", "frowning"},
		{"😮", "open mouth"}, {"😯", "hushed"}, {"😲", "astonished"}, {"😳", "flushed"},
		{"🥺", "pleading"}, {"🥹", "holding tears"}, {"😦", "frowning open"}, {"😧", "anguished"},
		{"😨", "fearful"}, {"😰", "anxious"}, {"😥", "sad relieved"}, {"😢", "crying"},
		{"😭", "sobbing"}, {"😱", "screaming"}, {"😖", "confounded"}, {"😣", "persevere"},
		{"😞", "disappointed"}, {"😓", "downcast"}, {"😩", "weary"}, {"😫", "tired"},
		{"🥱", "yawning"}, {"😤", "angry triumph"}, {"😡", "angry"}, {"😠", "pouting"},
		{"🤬", "cursing"}, {"😈", "smiling devil"}, {"👿", "angry devil"}, {"💀", "skull"},
		{"☠️", "skull and crossbones"}, {"💩", "poop"}, {"🤡", "clown"}, {"👹", "ogre"},
		{"👺", "goblin"}, {"👻", "ghost"}, {"👽", "alien"}, {"👾", "space invader"},
		{"🤖", "robot"},
	}},
	{"Gestures", [][2]string{
		{"👋", "waving"}, {"🤚", "raised back of hand"}, {"🖐️", "hand splayed"},
		{"✋", "raised hand"}, {"🖖", "vulcan salute"}, {"🫱", "rightwards hand"},
		{"🫲", "leftwards hand"}, {"🫳", "palm down"}, {"🫴", "palm up"},
		{"👌", "ok hand"}, {"🤌", "pinched fingers"}, {"🤏", "pinching hand"},
		{"✌️", "victory"}, {"🤞", "crossed fingers"}, {"🫰", "hand with index and thumb crossed"},
		{"🤟", "love you"}, {"🤘", "sign of horns"}, {"🤙", "call me"},
		{"👈", "pointing left"}, {"👉", "pointing right"}, {"👆", "pointing up"},
		{"🖕", "middle finger"}, {"👇", "pointing down"}, {"☝️", "index pointing up"},
		{"🫵", "index pointing away"}, {"👍", "thumbs up"}, {"👎", "thumbs down"},
		{"✊", "fist"}, {"👊", "punch"}, {"🤛", "left fist"}, {"🤜", "right fist"},
		{"👏", "clapping"}, {"🙌", "raising hands"}, {"🫶", "heart hands"},
		{"👐", "open hands"}, {"🤲", "palms up"}, {"🤝", "handshake"},
		{"🙏", "folded hands"}, {"✍️", "writing hand"}, {"💅", "nail polish"},
	}},
	{"Hearts", [][2]string{
		{"❤️", "red heart"}, {"🧡", "orange heart"}, {"💛", "yellow heart"},
		{"💚", "green heart"}, {"💙", "blue heart"}, {"💜", "purple heart"},
		{"🖤", "black heart"}, {"🤍", "white heart"}, {"🤎", "brown heart"},
		{"💔", "broken heart"}, {"❤️‍🔥", "heart on fire"}, {"❤️‍🩹", "mending heart"},
		{"💕", "two hearts"}, {"💞", "revolving hearts"}, {"💓", "beating heart"},
		{"💗", "growing heart"}, {"💖", "sparkling heart"}, {"💘", "cupid"},
		{"💝", "gift heart"}, {"💟", "heart decoration"},
	}},
	{"Objects", [][2]string{
		{"⭐", "star"}, {"🌟", "glowing star"}, {"✨", "sparkles"}, {"💫", "dizzy star"},
		{"🔥", "fire"}, {"💯", "hundred"}, {"💥", "boom"}, {"💫", "dizzy"},
		{"🎉", "party"}, {"🎊", "confetti"}, {"🎈", "balloon"}, {"🎁", "gift"},
		{"🏆", "trophy"}, {"🥇", "gold medal"}, {"⚡", "lightning"}, {"💎", "gem"},
		{"🔔", "bell"}, {"📎", "paperclip"}, {"📌", "pushpin"}, {"✅", "check mark"},
		{"❌", "cross mark"}, {"⭕", "circle"}, {"❗", "exclamation"}, {"❓", "question"},
		{"⏰", "alarm"}, {"📅", "calendar"}, {"📌", "pin"}, {"💡", "light bulb"},
	}},
	{"Nature", [][2]string{
		{"🌸", "cherry blossom"}, {"🌺", "hibiscus"}, {"🌻", "sunflower"}, {"🌹", "rose"},
		{"🌷", "tulip"}, {"🌱", "seedling"}, {"🌲", "evergreen tree"}, {"🌳", "deciduous tree"},
		{"🌴", "palm tree"}, {"🍀", "four leaf clover"}, {"🍁", "maple leaf"},
		{"🍂", "fallen leaf"}, {"🍃", "leaf fluttering"}, {"🌍", "earth"},
		{"🌙", "crescent moon"}, {"☀️", "sun"}, {"⭐", "star"}, {"🌈", "rainbow"},
	}},
	{"Food", [][2]string{
		{"🍎", "apple"}, {"🍊", "tangerine"}, {"🍋", "lemon"}, {"🍌", "banana"},
		{"🍉", "watermelon"}, {"🍇", "grapes"}, {"🍓", "strawberry"}, {"🫐", "blueberries"},
		{"🍑", "peach"}, {"🍒", "cherries"}, {"🥝", "kiwi"}, {"🍕", "pizza"},
		{"🍔", "hamburger"}, {"🍟", "fries"}, {"🌮", "taco"}, {"🍣", "sushi"},
		{"🍦", "ice cream"}, {"🍩", "donut"}, {"🍪", "cookie"}, {"🎂", "birthday cake"},
		{"☕", "coffee"}, {"🍵", "tea"}, {"🍺", "beer"}, {"🍷", "wine"},
	}},
	{"Travel", [][2]string{
		{"🚗", "car"}, {"🚕", "taxi"}, {"🚌", "bus"}, {"🚎", "trolley"},
		{"🚂", "locomotive"}, {"✈️", "airplane"}, {"🚀", "rocket"}, {"🛸", "flying saucer"},
		{"🏠", "house"}, {"🏢", "office"}, {"🏥", "hospital"}, {"🏫", "school"},
		{"🏨", "hotel"}, {"🚪", "door"}, {"🪟", "window"}, {"💡", "light bulb"},
	}},
}

type keyDef struct {
	label     string
	normal    string
	shifted   string
	key       string
	mod       string
	class     string
	action    string
	repeatKey bool
	special   bool
}

func (o *OSK) build() {
	root := gtk.NewOverlay()
	root.AddCSSClass("osk")
	root.SetHAlign(gtk.AlignFill)
	root.SetVAlign(gtk.AlignEnd)
	root.SetHExpand(true)

	// Content box holds the stack (keyboard / emoji / clipboard).
	content := gtk.NewBox(gtk.OrientationVertical, 0)
	content.SetHExpand(true)

	// Stack with three views.
	o.stack = gtk.NewStack()
	o.stack.SetHExpand(true)
	o.stack.SetVExpand(true)
	o.stack.SetTransitionDuration(150)
	o.stack.SetTransitionType(gtk.StackTransitionTypeSlideLeftRight)

	// Keyboard page.
	kbPage := gtk.NewBox(gtk.OrientationVertical, 0)
	kbPage.SetHExpand(true)
	o.buildKeyboard(kbPage)
	o.stack.AddNamed(kbPage, "keyboard")

	// Emoji page.
	emojiPage := o.buildEmojiPanel()
	o.stack.AddNamed(emojiPage, "emoji")

	// Clipboard page.
	clipboardPage := o.buildClipboardPanel()
	o.stack.AddNamed(clipboardPage, "clipboard")

	content.Append(o.stack)

	// Floating close button — top-right corner, overlaid.
	closeBtn := gtk.NewButton()
	closeBtn.AddCSSClass("osk-close-float")
	closeBtn.SetCursorFromName("pointer")
	closeLbl := gtkutil.MaterialIcon("close", "osk-key-label")
	closeBtn.SetChild(closeLbl)
	closeBtn.SetHAlign(gtk.AlignEnd)
	closeBtn.SetVAlign(gtk.AlignStart)
	closeBtn.ConnectClicked(func() {
		o.mu.Lock()
		o.manualOff = true
		o.manualMode = false
		publish := o.hideLocked()
		o.mu.Unlock()
		if publish {
			o.bus.Publish(bus.TopicOSKState, false)
		}
	})

	// Floating back button — top-left corner, shown only in panel views.
	backBtn := gtk.NewButton()
	backBtn.AddCSSClass("osk-close-float")
	backBtn.SetCursorFromName("pointer")
	backLbl := gtkutil.MaterialIcon("arrow_back", "osk-key-label")
	backBtn.SetChild(backLbl)
	backBtn.SetHAlign(gtk.AlignStart)
	backBtn.SetVAlign(gtk.AlignStart)
	backBtn.SetVisible(false)
	backBtn.ConnectClicked(func() {
		o.switchView("keyboard")
	})
	o.backBtn = backBtn

	root.SetChild(content)
	root.AddOverlay(closeBtn)
	root.AddOverlay(backBtn)
	o.win.SetChild(root)
	o.updateKeyLabels()

}
func (o *OSK) buildKeyboard(parent *gtk.Box) {
	numRow := []keyDef{
		{label: "Esc", key: "Escape", special: true},
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
		{label: "⌫", key: "BackSpace", class: "osk-key-wide", special: true},
	}

	row1 := []keyDef{
		{label: "Tab", key: "Tab", class: "osk-key-wide", special: true},
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
		{label: "Caps", action: "caps", class: "osk-key-wide", special: true},
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
		{label: "⏎", key: "Return", class: "osk-key-wide", special: true},
	}

	row3 := []keyDef{
		{label: "⇧", action: "shift", class: "osk-key-wide", special: true},
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
		{label: "⇧", action: "shift", class: "osk-key-wide", special: true},
	}

	row4 := []keyDef{
		{label: "Ctrl", mod: "Ctrl_L", class: "osk-key-wide", special: true},
		{label: "Alt", mod: "Alt_L", class: "osk-key-wide", special: true},
		{label: "emoji_emotions", action: "emoji", class: "osk-key-util-icon", special: true},
		{label: "", normal: " ", class: "osk-key-space"},
		{label: "content_paste", action: "clipboard", class: "osk-key-util-icon", special: true},
		{label: "←", key: "Left", class: "osk-key-arrow"},
		{label: "↓", key: "Down", class: "osk-key-arrow"},
		{label: "↑", key: "Up", class: "osk-key-arrow"},
		{label: "→", key: "Right", class: "osk-key-arrow"},
	}

	for _, row := range [][]keyDef{numRow, row1, row2, row3, row4} {
		o.buildRow(parent, row)
	}
}

func (o *OSK) buildEmojiPanel() gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("osk-panel")

	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("osk-panel-scroll")

	o.emojiContainer = gtk.NewBox(gtk.OrientationVertical, 0)
	o.emojiContainer.AddCSSClass("emoji-grid")
	o.emojiContainer.SetHAlign(gtk.AlignCenter)
	scroll.SetChild(o.emojiContainer)
	box.Append(scroll)

	o.populateEmojiGrid("")
	return box
}

func (o *OSK) populateEmojiGrid(query string) {
	gtkutil.ClearChildren(&o.emojiContainer.Widget, o.emojiContainer.Remove)

	for _, cat := range emojiData {
		var matched [][2]string
		for _, e := range cat.emojis {
			if query == "" || strings.Contains(strings.ToLower(e[1]), query) {
				matched = append(matched, e)
			}
		}
		if len(matched) == 0 {
			continue
		}
		header := gtk.NewLabel(cat.name)
		header.AddCSSClass("emoji-category-header")
		header.SetHAlign(gtk.AlignStart)
		o.emojiContainer.Append(header)

		flow := gtk.NewFlowBox()
		flow.SetColumnSpacing(4)
		flow.SetRowSpacing(4)
		for _, e := range matched {
			o.addEmojiBtn(flow, e[0], e[1])
		}
		o.emojiContainer.Append(flow)
	}
}

// copyAndPaste copies text to the clipboard via wl-copy, then types Ctrl+V.
// On failure, shows an error dialog. Switches back to keyboard view.
func (o *OSK) copyAndPaste(text string) {
	go func() {
		if err := exec.Command("wl-copy", text).Run(); err != nil {
			log.Printf("osk copy: %v", err)
			glib.IdleAdd(func() { gtkutil.ErrorDialog(o.win, "Copy failed", "Could not copy to clipboard.") })
			return
		}
		if o.ui != nil {
			time.Sleep(50 * time.Millisecond)
			o.ui.TypeChar("v", true, false)
		}
	}()
	o.switchView("keyboard")
}

func (o *OSK) addEmojiBtn(parent *gtk.FlowBox, char, name string) {
	btn := gtk.NewButton()
	btn.SetCursorFromName("pointer")
	btn.AddCSSClass("emoji-btn")
	lbl := gtk.NewLabel(char)
	lbl.AddCSSClass("emoji-char")
	btn.SetChild(lbl)
	btn.SetTooltipText(name)
	em := char
	btn.ConnectClicked(func() {
		o.copyAndPaste(em)
	})
	parent.Append(btn)
}

func (o *OSK) buildClipboardPanel() gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("osk-panel")

	// Scrollable list.
	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("osk-panel-scroll")

	o.clipboardList = gtk.NewBox(gtk.OrientationVertical, 4)
	o.clipboardList.AddCSSClass("osk-clipboard-list")
	scroll.SetChild(o.clipboardList)
	box.Append(scroll)

	return box
}

func (o *OSK) refreshClipboard(filter string) {
	go func() {
		out, err := exec.Command("cliphist", "list").Output()
		if err != nil {
			return
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		glib.IdleAdd(func() {
			gtkutil.ClearChildren(&o.clipboardList.Widget, o.clipboardList.Remove)

			for _, line := range lines {
				if line == "" {
					continue
				}
				if filter != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(filter)) {
					continue
				}

				// Display text: strip the cliphist ID prefix for the label.
				displayText := line
				if idx := strings.Index(line, "\t"); idx >= 0 {
					displayText = line[idx+1:]
				}

				row := gtk.NewButton()
				row.SetCursorFromName("pointer")
				row.AddCSSClass("clipboard-row")

				lbl := gtk.NewLabel(displayText)
				lbl.AddCSSClass("clipboard-preview")
				lbl.SetEllipsize(3)
				lbl.SetHAlign(gtk.AlignStart)
				lbl.SetXAlign(0)
				row.SetChild(lbl)

				// Capture line for closure.
				clipEntry := line
				row.ConnectClicked(func() {
					go func() {
						// Decode the cliphist entry to get actual clipboard content.
						decode := exec.Command("cliphist", "decode")
						decode.Stdin = strings.NewReader(clipEntry)
						decoded, err := decode.Output()
						if err != nil {
							log.Printf("clipboard decode: %v", err)
							return
						}
						if err := exec.Command("wl-copy", string(decoded)).Run(); err != nil {
							log.Printf("clipboard copy: %v", err)
							glib.IdleAdd(func() { gtkutil.ErrorDialog(o.win, "Copy failed", "Could not copy to clipboard.") })
							return
						}
						if o.ui != nil {
							time.Sleep(50 * time.Millisecond)
							o.ui.TypeChar("v", true, false)
						}
					}()
					o.switchView("keyboard")
				})

				o.clipboardList.Append(row)
			}
		})
	}()
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
		if d.special {
			btn.AddCSSClass("osk-key-special")
		}
		label := gtk.NewLabel(d.label)
		label.AddCSSClass("osk-key-label")
		btn.SetChild(label)

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
		case d.action == "emoji":
			o.emojiBtn = btn
			label.AddCSSClass("material-icon")
			gesture.ConnectReleased(func(int, float64, float64) {
				o.mu.Lock()
				defer o.mu.Unlock()
				if o.viewMode == "emoji" {
					o.switchView("keyboard")
				} else {
					o.switchView("emoji")
					o.showLocked()
				}
			})
		case d.action == "clipboard":
			o.clipboardBtn = btn
			label.AddCSSClass("material-icon")
			gesture.ConnectReleased(func(int, float64, float64) {
				o.mu.Lock()
				defer o.mu.Unlock()
				if o.viewMode == "clipboard" {
					o.switchView("keyboard")
				} else {
					o.switchView("clipboard")
					o.showLocked()
				}
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

	ch := o.activeCharLocked(kb)
	ctrl, alt := o.ctrlL, o.altL
	o.releaseAllModsLocked()
	o.mu.Unlock()
	o.updateKeyLabels()

	o.ui.TypeChar(ch, ctrl, alt)
}

// activeCharLocked returns the active character for the given key.
// Must be called with o.mu held.
func (o *OSK) activeCharLocked(kb *keyButton) string {
	shifted := o.shift != o.caps
	if shifted && kb.shifted != "" {
		return kb.shifted
	}
	return kb.normal
}

func (o *OSK) updateKeyLabels() {
	o.mu.Lock()
	shifted := o.shift != o.caps
	keys := make([]struct {
		label *gtk.Label
		ch    string
	}, len(o.keys))
	for i, k := range o.keys {
		ch := k.normal
		if shifted && k.shifted != "" {
			ch = k.shifted
		}
		keys[i].label = k.label
		keys[i].ch = ch
	}
	o.mu.Unlock()

	for _, k := range keys {
		lbl := k.label
		ch := k.ch
		glib.IdleAdd(func() { lbl.SetText(ch) })
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
		btn := btn
		if o.shift {
			glib.IdleAdd(func() { btn.AddCSSClass(cls) })
		} else {
			glib.IdleAdd(func() { btn.RemoveCSSClass(cls) })
		}
	}
	if o.capsBtn != nil {
		btn := o.capsBtn
		if o.caps {
			glib.IdleAdd(func() { btn.AddCSSClass(cls) })
		} else {
			glib.IdleAdd(func() { btn.RemoveCSSClass(cls) })
		}
	}
	for _, btn := range o.modBtns {
		if btn != nil {
			btn := btn
			glib.IdleAdd(func() { btn.RemoveCSSClass(cls) })
		}
	}
	if btn, ok := o.modBtns["Ctrl_L"]; ok {
		btn := btn
		if o.ctrlL {
			glib.IdleAdd(func() { btn.AddCSSClass(cls) })
		}
	}
	if btn, ok := o.modBtns["Alt_L"]; ok {
		btn := btn
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
