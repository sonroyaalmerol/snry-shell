// Package notifpopup provides notification toast popups.
package notifpopup

import (
	"time"

	"sync"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// NotifPopup shows floating notification toasts at the top-right.
type NotifPopup struct {
	win  *gtk.ApplicationWindow
	bus  *bus.Bus
	box  *gtk.Box
	dnd  bool
	dndMu sync.RWMutex
}

func New(app *gtk.Application, b *bus.Bus) *NotifPopup {
	win := gtk.NewApplicationWindow(app)
	win.SetDecorated(false)
	win.SetName("snry-notif-popup")

	layershell.InitForWindow(win)
	layershell.SetLayer(win, layershell.LayerOverlay)
	layershell.SetAnchor(win, layershell.EdgeTop, true)
	layershell.SetAnchor(win, layershell.EdgeRight, true)
	layershell.SetMargin(win, layershell.EdgeTop, 8)
	layershell.SetMargin(win, layershell.EdgeRight, 8)
	layershell.SetKeyboardMode(win, layershell.KeyboardModeNone)
	layershell.SetExclusiveZone(win, -1)
	layershell.SetNamespace(win, "snry-notif-popup")

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.SetHAlign(gtk.AlignEnd)
	box.SetVAlign(gtk.AlignStart)
	win.SetChild(box)

	p := &NotifPopup{win: win, bus: b, box: box}

	b.Subscribe(bus.TopicNotification, func(e bus.Event) {
		if e.Data == nil {
			return // dismiss event, ignore in popup
		}
		p.dndMu.RLock()
		active := p.dnd
		p.dndMu.RUnlock()
		if active {
			return
		}
		n := e.Data.(state.Notification)
		glib.IdleAdd(func() { p.AddToast(n) })
	})

	// Track DND state — suppress toasts when active.
	b.Subscribe(bus.TopicDND, func(e bus.Event) {
		if active, ok := e.Data.(bool); ok {
			p.dndMu.Lock()
			p.dnd = active
			p.dndMu.Unlock()
		}
	})

	win.SetVisible(false)
	return p
}

func (p *NotifPopup) AddToast(n state.Notification) {
	card := p.buildCard(n)
	revealer := gtk.NewRevealer()
	revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	revealer.SetTransitionDuration(250)
	revealer.SetChild(card)
	p.box.Prepend(revealer)
	p.win.SetVisible(true)

	glib.IdleAdd(func() { revealer.SetRevealChild(true) })

	timeout := 5000 * time.Millisecond
	if n.Timeout > 0 {
		timeout = time.Duration(n.Timeout) * time.Millisecond
	}
	time.AfterFunc(timeout, func() {
		glib.IdleAdd(func() { p.removeToast(revealer) })
	})
}

func (p *NotifPopup) removeToast(revealer *gtk.Revealer) {
	revealer.SetRevealChild(false)
	glib.TimeoutAdd(250, func() bool {
		p.box.Remove(&revealer.Widget)
		p.bus.Publish(bus.TopicNotification, nil) // dismiss event
		if p.box.FirstChild() == nil {
			p.win.SetVisible(false)
		}
		return false
	})
}

func (p *NotifPopup) buildCard(n state.Notification) gtk.Widgetter {
	card := gtk.NewBox(gtk.OrientationVertical, 4)
	card.AddCSSClass("notif-toast")
	if n.Urgency >= 2 {
		card.AddCSSClass("urgent")
	}

	header := gtk.NewBox(gtk.OrientationHorizontal, 0)
	header.SetHAlign(gtk.AlignFill)

	appName := gtk.NewLabel(n.AppName)
	appName.AddCSSClass("notif-toast-app-name")
	appName.SetHAlign(gtk.AlignStart)
	appName.SetHExpand(true)

	summary := gtk.NewLabel(n.Summary)
	summary.AddCSSClass("notif-toast-summary")
	summary.SetHAlign(gtk.AlignStart)

	header.Append(appName)
	card.Append(header)
	card.Append(summary)

	if n.Body != "" {
		body := gtk.NewLabel(n.Body)
		body.AddCSSClass("notif-toast-body")
		body.SetHAlign(gtk.AlignStart)
		body.SetWrap(true)
		card.Append(body)
	}
	return card
}
