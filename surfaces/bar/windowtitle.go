package bar

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

func newWindowTitleWidget(b *bus.Bus) gtk.Widgetter {
	label := gtk.NewLabel("")
	label.AddCSSClass("window-title")
	label.SetEllipsize(3) // pango.EllipsizeEnd
	label.SetMaxWidthChars(60)

	b.Subscribe(bus.TopicActiveWindow, func(e bus.Event) {
		win := e.Data.(state.ActiveWindow)
		glib.IdleAdd(func() {
			label.SetText(win.Title)
			label.SetTooltipText(win.Class)
		})
	})

	return label
}
