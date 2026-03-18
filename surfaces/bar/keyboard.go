package bar

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
)

func newKeyboardIndicator(b *bus.Bus, querier *hyprland.Querier) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 2)
	box.AddCSSClass("status-indicator")
	box.SetTooltipText("Keyboard layout")

	icon := gtk.NewLabel("language")
	icon.AddCSSClass("material-icon")

	label := gtk.NewLabel("")
	label.AddCSSClass("kbd-layout")

	box.Append(icon)
	box.Append(label)

	b.Subscribe(bus.TopicKeyboard, func(e bus.Event) {
		layout := e.Data.(string)
		glib.IdleAdd(func() { label.SetText(layout) })
	})

	// Click to cycle layout.
	clickGesture := gtk.NewGestureClick()
	clickGesture.SetButton(1)
	clickGesture.ConnectReleased(func(_ int, _ float64, _ float64) {
		_ = querier.SwitchXkbLayout()
	})
	box.AddController(clickGesture)

	return box
}
