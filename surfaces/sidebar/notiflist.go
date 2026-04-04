package sidebar

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const maxNotifications = 20

type notificationList struct {
	scroll *gtk.ScrolledWindow
	box    *gtk.Box
	bus    *bus.Bus
	count  int
}

func newNotificationList(b *bus.Bus) gtk.Widgetter {
	nl := &notificationList{
		scroll: gtk.NewScrolledWindow(),
		box:    gtk.NewBox(gtk.OrientationVertical, 4),
		bus:    b,
	}
	nl.box.AddCSSClass("notification-list")
	nl.scroll.SetChild(nl.box)
	nl.scroll.SetVExpand(true)
	nl.scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)

	b.Subscribe(bus.TopicNotification, func(e bus.Event) {
		if e.Data == nil {
			// Dismiss event — remove the most recent notification from sidebar.
			glib.IdleAdd(func() {
				nl.removeNewest()
			})
			return
		}
		n := e.Data.(state.Notification)
		glib.IdleAdd(func() {
			nl.prepend(n)
		})
	})

	return nl.scroll
}

// removeNewest removes the first (newest) notification card from the list.
func (nl *notificationList) removeNewest() {
	if first := nl.box.FirstChild(); first != nil {
		nl.box.Remove(first)
		if nl.count > 0 {
			nl.count--
		}
	}
}

func (nl *notificationList) prepend(n state.Notification) {
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
	card := gtk.NewBox(gtk.OrientationVertical, 4)
	card.AddCSSClass("notification-card")
	switch n.Urgency {
	case 0:
		card.AddCSSClass("urgency-low")
	case 2:
		card.AddCSSClass("urgency-critical")
	}

	header := gtk.NewBox(gtk.OrientationHorizontal, 8)

	appLabel := gtk.NewLabel(n.AppName)
	appLabel.AddCSSClass("notif-app")
	appLabel.SetHAlign(gtk.AlignStart)
	appLabel.SetHExpand(true)

	closeBtn := gtk.NewButton()
	closeBtn.AddCSSClass("notif-close")
	closeBtn.SetLabel("✕")
	closeBtn.ConnectClicked(func() {
		// Remove the card from the list on click.
		parent := card.Parent()
		if p, ok := parent.(*gtk.Box); ok {
			p.Remove(&card.Widget)
			nl.count--
		}
	})

	header.Append(appLabel)
	header.Append(closeBtn)

	summary := gtk.NewLabel(n.Summary)
	summary.AddCSSClass("notif-summary")
	summary.SetHAlign(gtk.AlignStart)
	summary.SetWrap(true)

	card.Append(header)
	card.Append(summary)

	if n.Body != "" {
		body := gtk.NewLabel(n.Body)
		body.AddCSSClass("notif-body")
		body.SetHAlign(gtk.AlignStart)
		body.SetWrap(true)
		body.SetTooltipText(fmt.Sprintf("ID: %d", n.ID))
		card.Append(body)
	}

	return card
}
