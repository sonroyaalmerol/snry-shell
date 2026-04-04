package surfaceutil

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
)

func AddEscapeToClose(win *gtk.ApplicationWindow) {
	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.ConnectKeyPressed(func(keyval, keycode uint, _ gdk.ModifierType) bool {
		if keyval == 0xff1b {
			win.SetVisible(false)
			return true
		}
		return false
	})
	win.AddController(keyCtrl)
}

func AddEscapeToCloseWithCallback(win *gtk.ApplicationWindow, onClose func()) {
	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.ConnectKeyPressed(func(keyval, keycode uint, _ gdk.ModifierType) bool {
		if keyval == 0xff1b {
			onClose()
			win.SetVisible(false)
			return true
		}
		return false
	})
	win.AddController(keyCtrl)
}

func AddToggleOn(b *bus.Bus, win *gtk.ApplicationWindow, action string) {
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == action {
			glib.IdleAdd(func() { win.SetVisible(!win.Visible()) })
		}
	})
}

func AddToggleOnWithFocus(b *bus.Bus, win *gtk.ApplicationWindow, action string) {
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == action {
			glib.IdleAdd(func() {
				win.SetVisible(!win.Visible())
				if win.Visible() {
					win.GrabFocus()
				}
			})
		}
	})
}

func FormatTime(seconds float64) string {
	s := int(seconds)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
