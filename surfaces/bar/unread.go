package bar

import (
	"strconv"
	"sync/atomic"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// newUnreadWidget creates a notification unread count badge.
func newUnreadWidget(b *bus.Bus) gtk.Widgetter {
	var count atomic.Int32

	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.SetVAlign(gtk.AlignCenter)

	icon := gtk.NewLabel("notifications")
	icon.AddCSSClass("material-icon")
	icon.AddCSSClass("status-indicator")

	badge := gtk.NewLabel("")
	badge.AddCSSClass("notif-unread-badge")
	badge.SetVisible(false)

	box.Append(icon)
	box.Append(badge)

	updateBadge := func() {
		c := int(count.Load())
		if c <= 0 {
			badge.SetVisible(false)
			return
		}
		if c > 99 {
			c = 99
		}
		badge.SetLabel(strconv.Itoa(c))
		badge.SetVisible(true)
	}

	b.Subscribe(bus.TopicNotification, func(e bus.Event) {
		if e.Data == nil {
			// Dismiss event — decrement.
			count.Add(-1)
		} else if _, ok := e.Data.(state.Notification); ok {
			// New notification — increment.
			count.Add(1)
		}
		updateBadge()
	})

	return box
}
