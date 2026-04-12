// Package session provides the session power menu (lock, suspend, reboot,
// shutdown, logout).
package session

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// Session is a power menu overlay.
type Session struct {
	win *gtk.ApplicationWindow
	bus *bus.Bus
}

func New(app *gtk.Application, b *bus.Bus) *Session {
	win := surfaceutil.NewFullscreenOverlay(app, "snry-session", layershell.KeyboardModeExclusive)

	s := &Session{win: win, bus: b}
	s.build()

	surfaceutil.AddToggleOnWithFocus(b, win, "toggle-session")
	surfaceutil.AddEscapeToClose(win)
	win.SetVisible(false)
	return s
}

func (s *Session) build() {
	// Dark scrim that covers the entire screen.
	scrim := gtk.NewBox(gtk.OrientationVertical, 0)
	scrim.AddCSSClass("session-scrim")
	scrim.SetHExpand(true)
	scrim.SetVExpand(true)

	// Click scrim to dismiss.
	clickGesture := gtk.NewGestureClick()
	clickGesture.SetButton(1)
	clickGesture.SetPropagationLimit(gtk.LimitNone)
	clickGesture.ConnectReleased(func(_ int, _ float64, _ float64) {
		s.win.SetVisible(false)
	})
	scrim.AddController(clickGesture)

	// Centered button container.
	centerBox := gtk.NewBox(gtk.OrientationVertical, 0)
	centerBox.SetHAlign(gtk.AlignCenter)
	centerBox.SetVAlign(gtk.AlignCenter)
	centerBox.SetVExpand(true)

	container := gtk.NewBox(gtk.OrientationHorizontal, 0)
	container.AddCSSClass("session-container")
	container.SetHAlign(gtk.AlignCenter)
	centerBox.Append(container)

	actions := []struct {
		action state.SessionAction
		icon   string
		label  string
		busCmd string // published to TopicSystemControls; "" = handled inline
	}{
		{state.SessionLock, "lock", "Lock", ""},
		{state.SessionSuspend, "bedtime", "Sleep", "system-suspend"},
		{state.SessionReboot, "restart_alt", "Reboot", "system-reboot"},
		{state.SessionShutdown, "power_settings_new", "Power off", "system-poweroff"},
		{state.SessionLogout, "logout", "Log out", "system-logout"},
	}

	for _, a := range actions {
		btn := s.buildBtn(a)
		container.Append(btn)
	}

	scrim.Append(centerBox)
	s.win.SetChild(scrim)
}

func (s *Session) buildBtn(a struct {
	action state.SessionAction
	icon   string
	label  string
	busCmd string
}) *gtk.Button {
	btn := gtkutil.M3FilledButton(a.label, "session-btn")

	inner := gtk.NewBox(gtk.OrientationVertical, 0)
	inner.SetHAlign(gtk.AlignCenter)
	inner.SetVAlign(gtk.AlignCenter)

	icon := gtkutil.MaterialIcon(a.icon)
	icon.SetHAlign(gtk.AlignCenter)

	label := gtk.NewLabel(a.label)
	label.AddCSSClass("session-btn-label")
	label.SetHAlign(gtk.AlignCenter)

	inner.Append(icon)
	inner.Append(label)
	btn.SetChild(inner)

	action := a
	btn.ConnectClicked(func() {
		s.bus.Publish(bus.TopicSessionAction, action.action)
		s.win.SetVisible(false)

		if action.action == state.SessionLock {
			s.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: true})
			return
		}

		busCmd := action.busCmd
		go func() {
			time.Sleep(200 * time.Millisecond)
			s.bus.Publish(bus.TopicSystemControls, busCmd)
		}()
	})
	return btn
}
