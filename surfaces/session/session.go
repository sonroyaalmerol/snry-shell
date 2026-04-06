// Package session provides the session power menu (lock, suspend, reboot,
// shutdown, logout).
package session

import (
	"log"
	"os/exec"
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
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:         "snry-session",
		Layer:        layershell.LayerOverlay,
		Anchors:      layershell.FullscreenAnchors(),
		KeyboardMode: layershell.KeyboardModeExclusive,
		Namespace:    "snry-session",
	})

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
		cmd    []string
	}{
		{state.SessionLock, "lock", "Lock", []string{"loginctl", "lock-session"}},
		{state.SessionSuspend, "bedtime", "Sleep", []string{"systemctl", "suspend"}},
		{state.SessionReboot, "restart_alt", "Reboot", []string{"systemctl", "reboot"}},
		{state.SessionShutdown, "power_settings_new", "Power off", []string{"systemctl", "poweroff"}},
		{state.SessionLogout, "logout", "Log out", []string{"hyprctl", "dispatch", "exit"}},
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
	cmd    []string
}) *gtk.Button {
	btn := gtkutil.M3FilledButton(a.label, "session-btn")

	inner := gtk.NewBox(gtk.OrientationVertical, 0)
	inner.SetHAlign(gtk.AlignCenter)
	inner.SetVAlign(gtk.AlignCenter)

	icon := gtk.NewLabel(a.icon)
	icon.AddCSSClass("material-icon")
	icon.SetHAlign(gtk.AlignCenter)

	label := gtk.NewLabel(a.label)
	label.AddCSSClass("session-btn-label")
	label.SetHAlign(gtk.AlignCenter)

	inner.Append(icon)
	inner.Append(label)
	btn.SetChild(inner)

	action := a
	cmd := a.cmd
	btn.ConnectClicked(func() {
		s.bus.Publish(bus.TopicSessionAction, action.action)
		s.win.SetVisible(false)
		go func() {
			time.Sleep(200 * time.Millisecond)
			if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
				log.Printf("session: %s: %v", cmd[0], err)
			}
		}()
	})
	return btn
}
