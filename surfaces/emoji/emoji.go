// Package emoji provides a standalone emoji picker overlay.
package emoji

import (
	"os/exec"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
)

// Picker is a bottom-center emoji picker overlay.
type Picker struct {
	win    *gtk.ApplicationWindow
	search *gtk.SearchEntry
	grid   *gtk.FlowBox
	bus    *bus.Bus
}

func New(app *gtk.Application, b *bus.Bus) *Picker {
	win := gtk.NewApplicationWindow(app)
	win.SetDecorated(false)
	win.SetName("snry-emoji-picker")

	layershell.InitForWindow(win)
	layershell.SetLayer(win, layershell.LayerOverlay)
	layershell.SetAnchor(win, layershell.EdgeBottom, true)
	layershell.SetMargin(win, layershell.EdgeBottom, 60)
	layershell.SetKeyboardMode(win, layershell.KeyboardModeOnDemand)
	layershell.SetExclusiveZone(win, -1)
	layershell.SetNamespace(win, "snry-emoji-picker")

	root := gtk.NewBox(gtk.OrientationVertical, 0)
	root.AddCSSClass("emoji-picker")
	root.SetSizeRequest(400, 350)

	search := gtk.NewSearchEntry()
	search.SetPlaceholderText("Search emoji...")
	search.SetHExpand(true)
	search.SetMarginTop(8)
	search.SetMarginStart(12)
	search.SetMarginEnd(12)
	search.ConnectSearchChanged(func() {
		filterGrid(search.Text())
	})
	p := &Picker{win: win, search: search}

	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetMarginBottom(8)

	grid := gtk.NewFlowBox()
	grid.AddCSSClass("emoji-grid")
	scroll.SetChild(grid)
	p.grid = grid

	root.Append(search)
	root.Append(scroll)
	win.SetChild(root)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-emoji" {
			glib.IdleAdd(func() {
				if win.Visible() {
					win.SetVisible(false)
				} else {
					populateGrid(grid)
					search.SetText("")
					win.SetVisible(true)
					search.GrabFocus()
				}
			})
		}
	})

	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.ConnectKeyPressed(func(keyval, keycode uint, state gdk.ModifierType) bool {
		if keyval == 0xff1b {
			win.SetVisible(false)
			return true
		}
		return false
	})
	win.AddController(keyCtrl)
	win.SetVisible(false)
	return p
}

func emojiButton(parent *gtk.FlowBox, emoji, name string) {
	btn := gtk.NewButton()
	btn.AddCSSClass("emoji-btn")
	lbl := gtk.NewLabel(emoji)
	lbl.AddCSSClass("emoji-char")
	btn.SetChild(lbl)
	btn.SetTooltipText(name)
	btn.ConnectClicked(func() {
		go func() { _ = exec.Command("wl-copy", emoji).Run() }()
	})
	parent.Append(btn)
}

func populateGrid(grid *gtk.FlowBox) {
	// Remove old children.
	var children []gtk.Widgetter
	for child := grid.FirstChild(); child != nil; {
		children = append(children, child)
		child = child.(*gtk.Widget).NextSibling()
	}
	for _, c := range children {
		grid.Remove(c)
	}

	categories := map[string][]struct {
		e string
		n string
	}{
		"Smileys": {
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
		},
		"Gestures": {
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
		},
		"Hearts": {
			{"❤️", "red heart"}, {"🧡", "orange heart"}, {"💛", "yellow heart"},
			{"💚", "green heart"}, {"💙", "blue heart"}, {"💜", "purple heart"},
			{"🖤", "black heart"}, {"🤍", "white heart"}, {"🤎", "brown heart"},
			{"💔", "broken heart"}, {"❤️‍🔥", "heart on fire"}, {"❤️‍🩹", "mending heart"},
			{"💕", "two hearts"}, {"💞", "revolving hearts"}, {"💓", "beating heart"},
			{"💗", "growing heart"}, {"💖", "sparkling heart"}, {"💘", "cupid"},
			{"💝", "gift heart"}, {"💟", "heart decoration"},
		},
		"Objects": {
			{"⭐", "star"}, {"🌟", "glowing star"}, {"✨", "sparkles"}, {"💫", "dizzy star"},
			{"🔥", "fire"}, {"💯", "hundred"}, {"💥", "boom"}, {"💫", "dizzy"},
			{"🎉", "party"}, {"🎊", "confetti"}, {"🎈", "balloon"}, {"🎁", "gift"},
			{"🏆", "trophy"}, {"🥇", "gold medal"}, {"⚡", "lightning"}, {"💎", "gem"},
			{"🔔", "bell"}, {"📎", "paperclip"}, {"📌", "pushpin"}, {"✅", "check mark"},
			{"❌", "cross mark"}, {"⭕", "circle"}, {"❗", "exclamation"}, {"❓", "question"},
			{"⏰", "alarm"}, {"📅", "calendar"}, {"📌", "pin"}, {"💡", "light bulb"},
		},
		"Nature": {
			{"🌸", "cherry blossom"}, {"🌺", "hibiscus"}, {"🌻", "sunflower"}, {"🌹", "rose"},
			{"🌷", "tulip"}, {"🌱", "seedling"}, {"🌲", "evergreen tree"}, {"🌳", "deciduous tree"},
			{"🌴", "palm tree"}, {"🍀", "four leaf clover"}, {"🍁", "maple leaf"},
			{"🍂", "fallen leaf"}, {"🍃", "leaf fluttering"}, {"🌍", "earth"},
			{"🌙", "crescent moon"}, {"☀️", "sun"}, {"⭐", "star"}, {"🌈", "rainbow"},
		},
		"Food": {
			{"🍎", "apple"}, {"🍊", "tangerine"}, {"🍋", "lemon"}, {"🍌", "banana"},
			{"🍉", "watermelon"}, {"🍇", "grapes"}, {"🍓", "strawberry"}, {"🫐", "blueberries"},
			{"🍑", "peach"}, {"🍒", "cherries"}, {"🥝", "kiwi"}, {"🍕", "pizza"},
			{"🍔", "hamburger"}, {"🍟", "fries"}, {"🌮", "taco"}, {"🍣", "sushi"},
			{"🍦", "ice cream"}, {"🍩", "donut"}, {"🍪", "cookie"}, {"🎂", "birthday cake"},
			{"☕", "coffee"}, {"🍵", "tea"}, {"🍺", "beer"}, {"🍷", "wine"},
		},
		"Travel": {
			{"🚗", "car"}, {"🚕", "taxi"}, {"🚌", "bus"}, {"🚎", "trolley"},
			{"🚂", "locomotive"}, {"✈️", "airplane"}, {"🚀", "rocket"}, {"🛸", "flying saucer"},
			{"🏠", "house"}, {"🏢", "office"}, {"🏥", "hospital"}, {"🏫", "school"},
			{"🏨", "hotel"}, {"🚪", "door"}, {"🪟", "window"}, {"💡", "light bulb"},
		},
	}

	for cat, emojis := range categories {
		header := gtk.NewLabel(cat)
		header.AddCSSClass("emoji-category-header")
		grid.Append(header)

		for _, e := range emojis {
			emojiButton(grid, e.e, e.n)
		}
	}
}

func filterGrid(query string) {
	// For simplicity, re-populate with filtered results.
	// Full implementation would hide/show existing widgets.
	_ = strings.ToLower(query)
}
