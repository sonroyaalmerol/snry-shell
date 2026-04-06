package bar

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

var newWindowTitleTrigger gtk.Widgetter

func newWindowTitleWidget(b *bus.Bus, q *hyprland.Querier, mon *gdk.Monitor) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("window-title-box")
	box.AddCSSClass("bar-group-clickable")
	box.SetCursorFromName("pointer")

	click := gtk.NewGestureClick()
	click.SetButton(1)
	click.ConnectReleased(func(_ int, _ float64, _ float64) {
		b.Publish(bus.TopicPopupTrigger, surfaceutil.PopupTrigger{
			Action: "toggle-windowmgmt", Trigger: box, Monitor: mon,
		})
		b.Publish(bus.TopicSystemControls, "toggle-windowmgmt")
	})
	box.AddController(click)

	newWindowTitleTrigger = box

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
