package bar

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

func newWindowTitleWidget(b *bus.Bus) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 6)
	box.AddCSSClass("window-title-box")

	classLabel := gtk.NewLabel("")
	classLabel.AddCSSClass("window-class")
	classLabel.SetEllipsize(3) // pango.EllipsizeEnd
	classLabel.SetMaxWidthChars(20)

	titleLabel := gtk.NewLabel("")
	titleLabel.AddCSSClass("window-title")
	titleLabel.SetEllipsize(3) // pango.EllipsizeEnd
	titleLabel.SetMaxWidthChars(40)

	box.Append(classLabel)
	box.Append(titleLabel)

	b.Subscribe(bus.TopicActiveWindow, func(e bus.Event) {
		win := e.Data.(state.ActiveWindow)
		glib.IdleAdd(func() {
			classLabel.SetText(win.Class)
			titleLabel.SetText(win.Title)
		})
	})

	return box
}
