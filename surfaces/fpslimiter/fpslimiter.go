// Package fpslimiter provides an FPS limiter overlay for Hyprland.
package fpslimiter

import (
	"os/exec"
	"strconv"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
)

// Overlay is a centered overlay for setting the Hyprland FPS limit.
type Overlay struct {
	win    *gtk.ApplicationWindow
	scale  *gtk.Scale
	valueL *gtk.Label
	bus    *bus.Bus
}

func New(app *gtk.Application, b *bus.Bus) *Overlay {
	win := gtk.NewApplicationWindow(app)
	win.SetDecorated(false)
	win.SetName("snry-fps-limiter")

	layershell.InitForWindow(win)
	layershell.SetLayer(win, layershell.LayerOverlay)
	layershell.SetAnchor(win, layershell.EdgeTop, true)
	layershell.SetAnchor(win, layershell.EdgeBottom, true)
	layershell.SetAnchor(win, layershell.EdgeLeft, true)
	layershell.SetAnchor(win, layershell.EdgeRight, true)
	layershell.SetKeyboardMode(win, layershell.KeyboardModeExclusive)
	layershell.SetExclusiveZone(win, -1)
	layershell.SetNamespace(win, "snry-fps-limiter")

	root := gtk.NewBox(gtk.OrientationVertical, 12)
	root.AddCSSClass("fps-limiter-overlay")
	root.SetHAlign(gtk.AlignCenter)
	root.SetVAlign(gtk.AlignCenter)
	root.SetSizeRequest(280, 120)

	title := gtk.NewLabel("FPS Limit")
	title.AddCSSClass("fps-limiter-title")

	o := &Overlay{win: win, bus: b}

	o.scale = gtk.NewScaleWithRange(gtk.OrientationHorizontal, 10, 144, 1)
	o.scale.AddCSSClass("fps-limiter-scale")
	o.scale.SetDrawValue(true)
	o.scale.SetValue(60)
	o.scale.SetHExpand(true)
	o.scale.SetMarginStart(16)
	o.scale.SetMarginEnd(16)

	o.valueL = gtk.NewLabel("60")
	o.valueL.AddCSSClass("fps-limiter-value")
	o.valueL.SetHAlign(gtk.AlignCenter)

	applyBtn := gtk.NewButtonWithLabel("Apply")
	applyBtn.AddCSSClass("fps-limiter-apply")
	applyBtn.SetHAlign(gtk.AlignCenter)
	applyBtn.ConnectClicked(func() {
		fps := int(o.scale.Value())
		o.valueL.SetText(strconv.Itoa(fps))
		go func() { _ = exec.Command("hyprctl", "setfps", strconv.Itoa(fps)).Run() }()
	})

	o.scale.ConnectChangeValue(func(_ gtk.ScrollType, value float64) bool {
		o.valueL.SetText(strconv.Itoa(int(value)))
		return false
	})

	root.Append(title)
	root.Append(o.scale)
	root.Append(applyBtn)
	win.SetChild(root)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-fps-limiter" {
			glib.IdleAdd(func() {
				if win.Visible() {
					win.SetVisible(false)
				} else {
					win.SetVisible(true)
					win.GrabFocus()
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
	return o
}
