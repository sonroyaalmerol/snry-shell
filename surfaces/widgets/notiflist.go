package widgets

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const maxNotifications = 20

type notificationList struct {
	box      *gtk.Box
	bus      *bus.Bus
	count    int
	empty    *gtk.Box
	hasNotif bool
}

func NewNotificationList(b *bus.Bus) gtk.Widgetter {
	nl := &notificationList{
		box: gtk.NewBox(gtk.OrientationVertical, 4),
		bus: b,
	}
	nl.box.AddCSSClass("notification-list")
	nl.box.SetVExpand(true)

	// Empty state placeholder.
	nl.empty = gtk.NewBox(gtk.OrientationVertical, 8)
	nl.empty.AddCSSClass("notification-empty")
	nl.empty.SetVExpand(true)
	nl.empty.SetVAlign(gtk.AlignCenter)
	nl.empty.SetHAlign(gtk.AlignCenter)
	icon := gtkutil.MaterialIcon("notifications_none")
	icon.AddCSSClass("notification-empty-icon")
	nl.empty.Append(icon)
	label := gtk.NewLabel("No notifications")
	label.AddCSSClass("notification-empty-label")
	nl.empty.Append(label)
	nl.box.Append(nl.empty)

	b.Subscribe(bus.TopicNotification, func(e bus.Event) {
		if e.Data == nil {
			return
		}
		n := e.Data.(state.Notification)
		glib.IdleAdd(func() {
			nl.prepend(n)
		})
	})

	return nl.box
}

func (nl *notificationList) prepend(n state.Notification) {
	if !nl.hasNotif {
		nl.empty.SetVisible(false)
		nl.hasNotif = true
	}
	card := nl.buildCard(n)
	nl.box.Prepend(card)
	nl.count++
	// Trim oldest if over limit.
	if nl.count > maxNotifications {
		if last := nl.box.LastChild(); last != nil {
			nl.box.Remove(last)
			nl.count--
		}
	}
}

func (nl *notificationList) buildCard(n state.Notification) gtk.Widgetter {
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("notification-card")
	switch n.Urgency {
	case 0:
		card.AddCSSClass("urgency-low")
	case 2:
		card.AddCSSClass("urgency-critical")
	}

	// Header row: app name and close button
	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	header.SetMarginBottom(4)

	appLabel := gtk.NewLabel(n.AppName)
	appLabel.AddCSSClass("notification-app-name")
	appLabel.SetHAlign(gtk.AlignStart)
	appLabel.SetHExpand(true)

	closeBtn := gtkutil.M3IconButton("close", "notification-dismiss-btn")
	closeBtn.ConnectClicked(func() {
		parent := card.Parent()
		if p, ok := parent.(*gtk.Box); ok {
			p.Remove(&card.Widget)
			nl.count--
			if nl.count == 0 {
				nl.empty.SetVisible(true)
				nl.hasNotif = false
			}
			nl.bus.Publish(bus.TopicNotification, nil)
		}
	})

	header.Append(appLabel)
	header.Append(closeBtn)

	// Summary
	summary := gtk.NewLabel(n.Summary)
	summary.AddCSSClass("notification-summary")
	summary.SetHAlign(gtk.AlignStart)
	summary.SetWrap(true)

	card.Append(header)
	card.Append(summary)

	// Body (if present)
	if n.Body != "" {
		body := gtk.NewLabel(n.Body)
		body.AddCSSClass("notification-body")
		body.SetHAlign(gtk.AlignStart)
		body.SetWrap(true)
		card.Append(body)
	}

	return card
}
