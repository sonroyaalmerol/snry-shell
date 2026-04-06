package bar

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

func newWindowTitleWidget(b *bus.Bus, q *hyprland.Querier) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("window-title-box")

	classLabel := gtk.NewLabel("")
	classLabel.AddCSSClass("window-class")
	classLabel.SetEllipsize(3) // pango.EllipsizeEnd
	classLabel.SetMaxWidthChars(30)

	titleLabel := gtk.NewLabel("")
	titleLabel.AddCSSClass("window-title")
	titleLabel.SetEllipsize(3) // pango.EllipsizeEnd
	titleLabel.SetMaxWidthChars(40)

	box.Append(classLabel)
	box.Append(titleLabel)

	updateLabels := func(win state.ActiveWindow) {
		classLabel.SetText(win.Class)
		titleLabel.SetText(win.Title)
	}

	// Seed initial state from hyprctl and re-query on workspace changes
	// to avoid stale empty titles from the activewindow event race.
	refresh := func() {
		if w, err := q.ActiveWindow(); err == nil {
			updateLabels(state.ActiveWindow{Class: w.Class, Title: w.Title})
		}
	}
	glib.IdleAdd(refresh)

	b.Subscribe(bus.TopicWorkspaces, func(_ bus.Event) {
		glib.IdleAdd(refresh)
	})

	b.Subscribe(bus.TopicActiveWindow, func(e bus.Event) {
		win := e.Data.(state.ActiveWindow)
		glib.IdleAdd(func() {
			updateLabels(win)
		})
	})

	return box
}
