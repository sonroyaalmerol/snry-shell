package bar

import (
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

	b.Subscribe(bus.TopicNotification, func(e bus.Event) {
		n, ok := e.Data.(state.Notification)
		if !ok {
			return
		}
		// Notification added: increment
		newCount := count.Add(1)
		showCount := int(newCount)
		if showCount > 99 {
			showCount = 99
		}
		_ = n.ID
		if showCount > 0 {
			badge.SetLabel(string(rune('0' + showCount/10)) + string(rune('0'+showCount%10)))
			badge.SetVisible(true)
		}
	})

	// Subscribe to notification dismiss/clear — we listen for a nil Notification
	// which signals the notification popup was dismissed.
	b.Subscribe(bus.TopicNotification, func(e bus.Event) {
		if e.Data == nil {
			if c := count.Add(-1); c < 0 {
				count.Store(0)
				badge.SetVisible(false)
			} else if c == 0 {
				badge.SetVisible(false)
			}
		}
	})

	return box
}
