package bar

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
)

func newClockWidget(b *bus.Bus) gtk.Widgetter {
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

	format := "15:04"

	update := func() {
		now := time.Now()
		timeLabel.SetText(now.Format(format))
		dateLabel.SetText(now.Format("Mon Jan 02"))
	}

	b.Subscribe(bus.TopicSettingsChanged, func(e bus.Event) {
		if cfg, ok := e.Data.(settings.Config); ok {
			glib.IdleAdd(func() {
				if cfg.ClockFormat == "12h" {
					format = "03:04 PM"
				} else {
					format = "15:04"
				}
				update()
			})
		}
	})

	update()

	glib.TimeoutAdd(1000, func() bool {
		update()
		return true
	})

	return box
}
