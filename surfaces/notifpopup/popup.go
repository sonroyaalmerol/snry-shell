// Package notifpopup provides notification toast popups.
package notifpopup

import (
	"sync/atomic"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// NotifPopup shows floating notification toasts.
type NotifPopup struct {
	win      *gtk.ApplicationWindow
	bus      *bus.Bus
	box      *gtk.Box
	dnd      atomic.Bool
	timeout  time.Duration
	position string
}

func New(app *gtk.Application, b *bus.Bus) *NotifPopup {
	p := &NotifPopup{
		bus:      b,
		timeout:  5000 * time.Millisecond,
		position: "top-right",
	}

	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-notif-popup",
		Layer:         layershell.LayerOverlay,
		Anchors:       map[layershell.Edge]bool{layershell.EdgeTop: true, layershell.EdgeRight: true},
		Margins:       map[layershell.Edge]int{layershell.EdgeTop: 8, layershell.EdgeRight: 8},
		KeyboardMode:  layershell.KeyboardModeNone,
		ExclusiveZone: -1,
		Namespace:     "snry-notif-popup",
	})

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	p.win = win
	p.box = box
	win.SetChild(box)

	b.Subscribe(bus.TopicNotification, func(e bus.Event) {
		if e.Data == nil {
			return // dismiss event, ignore in popup
		}
		if p.dnd.Load() {
			return
		}
		n := e.Data.(state.Notification)
		glib.IdleAdd(func() { p.AddToast(n) })
	})

	// Track DND state — suppress toasts when active.
	b.Subscribe(bus.TopicDND, func(e bus.Event) {
		if active, ok := e.Data.(bool); ok {
			p.dnd.Store(active)
		}
	})

	b.Subscribe(bus.TopicSettingsChanged, func(e bus.Event) {
		if cfg, ok := e.Data.(settings.Config); ok {
			glib.IdleAdd(func() {
				p.timeout = time.Duration(cfg.NotificationTimeout) * time.Millisecond
				if p.position != cfg.NotificationPosition {
					p.position = cfg.NotificationPosition
					p.updateLayout()
				}
			})
		}
	})

	win.SetVisible(false)
	return p
}

func (p *NotifPopup) updateLayout() {
	anchors := map[layershell.Edge]bool{}
	margins := map[layershell.Edge]int{}

	switch p.position {
	case "top-right":
		anchors[layershell.EdgeTop] = true
		anchors[layershell.EdgeRight] = true
		margins[layershell.EdgeTop] = 8
		margins[layershell.EdgeRight] = 8
		p.box.SetHAlign(gtk.AlignEnd)
		p.box.SetVAlign(gtk.AlignStart)
	case "top-left":
		anchors[layershell.EdgeTop] = true
		anchors[layershell.EdgeLeft] = true
		margins[layershell.EdgeTop] = 8
		margins[layershell.EdgeLeft] = 8
		p.box.SetHAlign(gtk.AlignStart)
		p.box.SetVAlign(gtk.AlignStart)
	case "bottom-right":
		anchors[layershell.EdgeBottom] = true
		anchors[layershell.EdgeRight] = true
		margins[layershell.EdgeBottom] = 8
		margins[layershell.EdgeRight] = 8
		p.box.SetHAlign(gtk.AlignEnd)
		p.box.SetVAlign(gtk.AlignEnd)
	case "bottom-left":
		anchors[layershell.EdgeBottom] = true
		anchors[layershell.EdgeLeft] = true
		margins[layershell.EdgeBottom] = 8
		margins[layershell.EdgeLeft] = 8
		p.box.SetHAlign(gtk.AlignStart)
		p.box.SetVAlign(gtk.AlignEnd)
	}

	for edge, val := range anchors {
		layershell.SetAnchor(p.win, edge, val)
	}
	for edge, val := range margins {
		layershell.SetMargin(p.win, edge, val)
	}
}

func (p *NotifPopup) AddToast(n state.Notification) {
	card := p.buildCard(n)
	revealer := gtk.NewRevealer()

	transition := gtk.RevealerTransitionTypeSlideDown
	if p.position == "bottom-right" || p.position == "bottom-left" {
		transition = gtk.RevealerTransitionTypeSlideUp
		p.box.Append(revealer)
	} else {
		p.box.Prepend(revealer)
	}

	revealer.SetTransitionType(transition)
	revealer.SetTransitionDuration(250)
	revealer.SetChild(card)

	p.win.SetVisible(true)
	glib.IdleAdd(func() { revealer.SetRevealChild(true) })

	timeout := p.timeout
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
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("notif-toast")
	if n.Urgency >= 2 {
		card.AddCSSClass("urgent")
	}

	// App name header
	appName := gtk.NewLabel(n.AppName)
	appName.AddCSSClass("notif-toast-app-name")
	appName.SetHAlign(gtk.AlignStart)
	card.Append(appName)

	// Summary
	summary := gtk.NewLabel(n.Summary)
	summary.AddCSSClass("notif-toast-summary")
	summary.SetHAlign(gtk.AlignStart)
	summary.SetWrap(true)
	card.Append(summary)

	// Body (if present)
	if n.Body != "" {
		body := gtk.NewLabel(n.Body)
		body.AddCSSClass("notif-toast-body")
		body.SetHAlign(gtk.AlignStart)
		body.SetWrap(true)
		card.Append(body)
	}
	return card
}
