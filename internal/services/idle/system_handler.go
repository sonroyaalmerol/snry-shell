package idle

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/dbusutil"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// SystemHandler handles hardware events like lid close and power button.
// It relies on Hyprland keybindings (registered externally via SetupHyprlandBinds)
// to receive the events, and a logind block inhibitor to suppress logind's default action.
type SystemHandler struct {
	bus  *bus.Bus
	conn dbusutil.DBusConn

	lidAction   atomic.Pointer[string]
	powerAction atomic.Pointer[string]

	// Inhibitor lock file descriptor
	lockFD dbus.UnixFD
}

func NewSystemHandler(b *bus.Bus, conn dbusutil.DBusConn, lidAction, powerAction string) *SystemHandler {
	h := &SystemHandler{
		bus:  b,
		conn: conn,
	}
	h.lidAction.Store(&lidAction)
	h.powerAction.Store(&powerAction)
	return h
}

func (h *SystemHandler) UpdateConfig(lidAction, powerAction string) {
	h.lidAction.Store(&lidAction)
	h.powerAction.Store(&powerAction)
	log.Printf("[system] config updated: lid=%s, power=%s", lidAction, powerAction)
}

func (h *SystemHandler) Run(ctx context.Context) error {
	realConn, ok := h.conn.(*dbusutil.RealConn)
	if !ok || realConn.Conn == nil {
		return fmt.Errorf("no system bus connection")
	}

	// Take a block inhibitor so logind does not handle these events themselves.
	// The actual detection is done via Hyprland bindl keybindings set up by
	// SetupHyprlandBinds, which send toggle-power-action / toggle-lid-action
	// through the control socket.
	fd, err := dbusutil.LogindInhibit(realConn.Conn, "handle-power-key:handle-lid-switch", "snry-shell", "Shell handling system buttons", "block")
	if err != nil {
		log.Printf("[system] failed to inhibit logind: %v", err)
	} else {
		h.lockFD = fd
		log.Printf("[system] logind button handling inhibited (block mode)")
	}

	// Receive power-action and lid-action commands forwarded from Hyprland bindings.
	h.bus.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		cmd, ok := e.Data.(string)
		if !ok {
			return
		}

		switch cmd {
		case "toggle-power-action":
			log.Printf("[system] power button press received")
			if p := h.powerAction.Load(); p != nil {
				h.executeAction(*p)
			}
		case "toggle-lid-action":
			log.Printf("[system] lid close received")
			if l := h.lidAction.Load(); l != nil {
				h.executeAction(*l)
			}
		}
	})

	<-ctx.Done()
	return ctx.Err()
}

func (h *SystemHandler) Suspend() {
	h.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: true})
	if realConn, ok := h.conn.(*dbusutil.RealConn); ok && realConn.Conn != nil {
		if err := dbusutil.LogindSuspend(realConn.Conn); err != nil {
			log.Printf("[system] logind Suspend: %v", err)
		}
	}
}

func (h *SystemHandler) Reboot() {
	if realConn, ok := h.conn.(*dbusutil.RealConn); ok && realConn.Conn != nil {
		if err := dbusutil.LogindReboot(realConn.Conn); err != nil {
			log.Printf("[system] logind Reboot: %v", err)
		}
	}
}

func (h *SystemHandler) PowerOff() {
	if realConn, ok := h.conn.(*dbusutil.RealConn); ok && realConn.Conn != nil {
		if err := dbusutil.LogindPowerOff(realConn.Conn); err != nil {
			log.Printf("[system] logind PowerOff: %v", err)
		}
	}
}

func (h *SystemHandler) executeAction(action string) {
	if action == "ignore" {
		return
	}

	log.Printf("[system] executing action: %s", action)
	switch action {
	case "lock":
		h.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: true})
	case "suspend":
		h.Suspend()
	case "shutdown":
		h.PowerOff()
	case "session-menu":
		h.bus.Publish(bus.TopicSystemControls, "toggle-session")
	}
}
