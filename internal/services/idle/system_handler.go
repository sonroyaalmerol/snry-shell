package idle

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// SystemHandler handles hardware events like lid close and power button.
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

	// 1. Inhibit logind's default handling.
	// We use "block" mode to completely prevent logind's default action.
	err := h.conn.Object("org.freedesktop.login1", "/org/freedesktop/login1").
		Call("org.freedesktop.login1.Manager.Inhibit", 0,
			"handle-power-key:handle-lid-switch", "snry-shell", "Shell handling system buttons", "block").
		Store(&h.lockFD)

	if err != nil {
		log.Printf("[SYSTEM] failed to inhibit logind: %v", err)
	} else {
		log.Printf("[SYSTEM] logind button handling inhibited (block mode)")
	}

	// 2. Listen for UPower signals (Lid state).
	_ = h.conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.UPower"),
		dbus.WithMatchMember("PropertiesChanged"),
	)

	// 3. Listen for logind signals.
	// We watch for signals from the Manager object explicitly.
	_ = h.conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
		dbus.WithMatchObjectPath("/org/freedesktop/login1"),
	)

	ch := make(chan *dbus.Signal, 16)
	h.conn.Signal(ch)
	defer h.conn.RemoveSignal(ch)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig := <-ch:
			h.handleSignal(sig)
		}
	}
}

func (h *SystemHandler) handleSignal(sig *dbus.Signal) {
	h.mu.Lock()
	lidAction := h.lidAction
	powerAction := h.powerAction
	h.mu.Unlock()

	// Handle button signals from logind
	if sig.Name == "org.freedesktop.login1.Manager.Button" || sig.Name == "org.freedesktop.login1.Manager.PowerKey" {
		isPower := false
		if sig.Name == "org.freedesktop.login1.Manager.PowerKey" {
			isPower = true
		} else if len(sig.Body) >= 1 {
			if name, ok := sig.Body[0].(string); ok && name == "power" {
				isPower = true
			}
		}

		if isPower {
			log.Printf("[SYSTEM] power button press detected")
			h.executeAction(powerAction)
			return
		}
	}

	// Handle lid signals
	if sig.Name == "org.freedesktop.login1.Manager.LidClosed" {
		log.Printf("[SYSTEM] logind lid closure detected")
		h.executeAction(lidAction)
		return
	}

	if sig.Name == "org.freedesktop.UPower.PropertiesChanged" {
		if len(sig.Body) >= 2 {
			props, ok := sig.Body[1].(map[string]dbus.Variant)
			if ok {
				if val, ok := props["LidIsClosed"]; ok {
					if closed, ok := val.Value().(bool); ok && closed {
						log.Printf("[SYSTEM] UPower lid closure detected")
						h.executeAction(lidAction)
					}
				}
			}
		}
	}
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
		go exec.Command("systemctl", "suspend").Run()
	case "shutdown":
		go exec.Command("systemctl", "poweroff").Run()
	case "session-menu":
		h.bus.Publish(bus.TopicSystemControls, "toggle-session")
	}
}
