package bar

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func newClockWidget() gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.AddCSSClass("bar-clock-box")
	box.SetHAlign(gtk.AlignCenter)
	box.SetVAlign(gtk.AlignCenter)

	timeLabel := gtk.NewLabel("")
	timeLabel.AddCSSClass("bar-clock")

	sepLabel := gtk.NewLabel("·")
	sepLabel.AddCSSClass("bar-clock-sep")

	dateLabel := gtk.NewLabel("")
	dateLabel.AddCSSClass("bar-date")

	box.Append(timeLabel)
	box.Append(sepLabel)
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
