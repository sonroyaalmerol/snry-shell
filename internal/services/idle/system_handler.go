package idle

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// SystemHandler handles hardware events like lid close and power button.
// It relies on Hyprland keybindings (registered externally via SetupHyprlandBinds)
// to receive the events, and a logind block inhibitor to suppress logind's default action.
type SystemHandler struct {
	bus  *bus.Bus
	conn *dbus.Conn
	mu   sync.Mutex

	lidAction   string
	powerAction string

	// Inhibitor lock file descriptor
	lockFD dbus.UnixFD
}

func NewSystemHandler(b *bus.Bus, conn *dbus.Conn, lidAction, powerAction string) *SystemHandler {
	return &SystemHandler{
		bus:         b,
		conn:        conn,
		lidAction:   lidAction,
		powerAction: powerAction,
	}
}

func (h *SystemHandler) UpdateConfig(lidAction, powerAction string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lidAction = lidAction
	h.powerAction = powerAction
	log.Printf("[SYSTEM] config updated: lid=%s, power=%s", lidAction, powerAction)
}

func (h *SystemHandler) Run(ctx context.Context) error {
	if h.conn == nil {
		return fmt.Errorf("no system bus connection")
	}

	// Take a block inhibitor so logind does not handle these events itself.
	// The actual detection is done via Hyprland bindl keybindings set up by
	// SetupHyprlandBinds, which send toggle-power-action / toggle-lid-action
	// through the control socket.
	err := h.conn.Object("org.freedesktop.login1", "/org/freedesktop/login1").
		Call("org.freedesktop.login1.Manager.Inhibit", 0,
			"handle-power-key:handle-lid-switch", "snry-shell", "Shell handling system buttons", "block").
		Store(&h.lockFD)
	if err != nil {
		log.Printf("[SYSTEM] failed to inhibit logind: %v", err)
	} else {
		log.Printf("[SYSTEM] logind button handling inhibited (block mode)")
	}

	// Receive power-action and lid-action commands forwarded from Hyprland bindings.
	h.bus.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		cmd, ok := e.Data.(string)
		if !ok {
			return
		}
		h.mu.Lock()
		lidAction := h.lidAction
		powerAction := h.powerAction
		h.mu.Unlock()

		switch cmd {
		case "toggle-power-action":
			log.Printf("[SYSTEM] power button press received")
			h.executeAction(powerAction)
		case "toggle-lid-action":
			log.Printf("[SYSTEM] lid close received")
			h.executeAction(lidAction)
		}
	})

	<-ctx.Done()
	return ctx.Err()
}

func (h *SystemHandler) executeAction(action string) {
	if action == "ignore" {
		return
	}

	log.Printf("[SYSTEM] executing action: %s", action)
	switch action {
	case "lock":
		h.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: true})
	case "suspend":
		h.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: true})
		go logindSuspend(h.conn)
	case "shutdown":
		go logindPowerOff(h.conn)
	case "session-menu":
		h.bus.Publish(bus.TopicSystemControls, "toggle-session")
	}
}

// ── logind D-Bus helpers ──────────────────────────────────────────────────────

func logindSuspend(conn *dbus.Conn) {
	if conn == nil {
		return
	}
	if err := conn.Object(logindDest, logindManager).
		Call(managerIface+".Suspend", 0, false).Err; err != nil {
		log.Printf("[SYSTEM] logind Suspend: %v", err)
	}
}

func logindReboot(conn *dbus.Conn) {
	if conn == nil {
		return
	}
	if err := conn.Object(logindDest, logindManager).
		Call(managerIface+".Reboot", 0, false).Err; err != nil {
		log.Printf("[SYSTEM] logind Reboot: %v", err)
	}
}

func logindPowerOff(conn *dbus.Conn) {
	if conn == nil {
		return
	}
	if err := conn.Object(logindDest, logindManager).
		Call(managerIface+".PowerOff", 0, false).Err; err != nil {
		log.Printf("[SYSTEM] logind PowerOff: %v", err)
	}
}
