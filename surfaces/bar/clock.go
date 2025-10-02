package bar

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func newClockWidget() gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("bar-clock-box")
	box.SetHAlign(gtk.AlignCenter)
	box.SetVAlign(gtk.AlignCenter)

	timeLabel := gtk.NewLabel("")
	timeLabel.AddCSSClass("bar-clock")
	timeLabel.SetHAlign(gtk.AlignCenter)

	dateLabel := gtk.NewLabel("")
	dateLabel.AddCSSClass("bar-date")
	dateLabel.SetHAlign(gtk.AlignCenter)

	box.Append(timeLabel)
	box.Append(dateLabel)

	update := func() {
		now := time.Now()
		timeLabel.SetText(now.Format("15:04"))
		dateLabel.SetText(now.Format("Mon Jan 02"))
	}
	update()

	glib.TimeoutAdd(1000, func() bool {
		update()
		return true
	})

	return box
}
