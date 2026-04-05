// Package atspi2 watches for text input focus changes via the AT-SPI2
// accessibility D-Bus bus. When a widget with a text role (entry, terminal,
// text editor, etc.) gains keyboard focus, it publishes a TopicTextInputFocus
// event. This provides per-field precision rather than app-level heuristics.
package atspi2

import (
	"context"
	"log"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
)

// AT-SPI2 role constants for text input widgets.
const (
	roleEntry       = 70 // single-line text entry
	rolePasswordText = 38 // password entry
	roleTerminal    = 58 // terminal emulator
	roleText        = 59 // multi-line text editor
	roleSpinButton  = 50 // numeric spin button
	roleParagraph   = 41 // paragraph
	roleEditbar     = 68 // editable text in toolbar
	roleDateEditor  = 27 // date entry
)

// getA11yBusAddr queries the session bus for the accessibility bus address.
// If the bus isn't running yet, it attempts to auto-start it via DBus activation
// and retries with a short backoff.
func getA11yBusAddr(sess *dbus.Conn) (string, error) {
	var busAddr string
	err := sess.Object("org.a11y.Bus", "/org/a11y/Bus").
		Call("org.a11y.Bus.GetAddress", 0).Store(&busAddr)
	if err == nil {
		return busAddr, nil
	}

	// Attempt to auto-start the a11y bus via DBus activation.
	log.Printf("[ATSPI2] bus not running, attempting auto-start")
	startErr := sess.Object("org.freedesktop.DBus", "/org/freedesktop/DBus").
		Call("org.freedesktop.DBus.StartServiceByName", 0, "org.a11y.Bus").Store(nil)
	if startErr != nil {
		return "", err
	}

	// The daemon may need a moment to register on the bus.
	for delay := 100 * time.Millisecond; delay <= 3*time.Second; delay *= 2 {
		time.Sleep(delay)
		err = sess.Object("org.a11y.Bus", "/org/a11y/Bus").
			Call("org.a11y.Bus.GetAddress", 0).Store(&busAddr)
		if err == nil {
			log.Printf("[ATSPI2] a11y bus started after %v", delay)
			return busAddr, nil
		}
	}

	return "", err
}

var textRoles = map[uint32]bool{
	roleEntry: true, rolePasswordText: true, roleTerminal: true,
	roleText: true, roleSpinButton: true, roleParagraph: true,
	roleEditbar: true, roleDateEditor: true,
}

// Watcher monitors AT-SPI2 StateChanged:focused events on the
// accessibility bus and publishes TopicTextInputFocus events.
type Watcher struct {
	conn    *dbus.Conn
	bus     *bus.Bus
	healthy bool // true once at least one event has been received
}

// New creates a Watcher. Returns nil, nil if AT-SPI2 is not available
// (no accessibility bus, registry missing, etc.). The caller should still
// use the window class heuristic as a fallback.
func New(b *bus.Bus) (*Watcher, error) {
	// Step 1: get the accessibility bus address from the session bus.
	sess, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Printf("[ATSPI2] cannot connect to session bus: %v", err)
		return nil, nil
	}
	defer sess.Close()

	busAddr, err := getA11yBusAddr(sess)
	if err != nil {
		log.Printf("[ATSPI2] cannot get a11y bus address: %v", err)
		return nil, nil
	}

	// Step 2: connect to the accessibility bus.
	conn, err := dbus.Dial(busAddr)
	if err != nil {
		log.Printf("[ATSPI2] cannot connect to a11y bus %s: %v", busAddr, err)
		return nil, nil
	}

	// Step 3: verify the registry exists.
	obj := conn.Object("org.a11y.atspi.Registry", "/org/a11y/atspi/registry")
	var introspect string
	err = obj.Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&introspect)
	if err != nil {
		log.Printf("[ATSPI2] a11y registry not available: %v", err)
		conn.Close()
		return nil, nil
	}

	// Step 4: subscribe to StateChanged signals.
	err = conn.AddMatchSignal(
		dbus.WithMatchInterface("org.a11y.atspi.Event.Object"),
		dbus.WithMatchMember("StateChanged"),
	)
	if err != nil {
		conn.Close()
		return nil, err
	}

	log.Printf("[ATSPI2] connected to accessibility bus at %s", busAddr)
	return &Watcher{conn: conn, bus: b}, nil
}

// Healthy returns true if the watcher has received at least one event,
// indicating AT-SPI2 is actively delivering focus events.
func (w *Watcher) Healthy() bool {
	return w.healthy
}

// Run listens for AT-SPI2 focus events and publishes to the bus.
// Should be called in a goroutine. Blocks until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	ch := make(chan *dbus.Signal, 100)
	w.conn.Signal(ch)

	log.Printf("[ATSPI2] watching for StateChanged:focused events")

	// Health check: if no events arrive within 5s, AT-SPI2 is likely
	// not working (apps not registered, bus misconfigured).
	healthTimer := time.AfterFunc(5*time.Second, func() {
		if !w.healthy {
			log.Printf("[ATSPI2] no events received after 5s, marking as unhealthy")
		}
	})

	defer func() {
		healthTimer.Stop()
		w.conn.Close()
	}()

	for {
		select {
		case sig := <-ch:
			w.handleSignal(sig)
		case <-ctx.Done():
			return
		}
	}
}

func (w *Watcher) handleSignal(sig *dbus.Signal) {
	// AT-SPI2 signals have format ((so)(so)(si)sv(ii)siibbbbb):
	//
	//   Body[0]: (so) source application [bus_name, object_path]
	//   Body[1]: (so) source object      [bus_name, object_path]
	//   Body[2]: (si) event detail       [name, detail]
	//   Body[3]: sv   any_data          (variant)
	//   Body[4]: (ii) screen coords    [x, y]
	//   Body[5]: s    state name        "focused"
	//   Body[6]: i    state value       1=gained, 0=lost
	//   Body[7]: i    old value
	//   Body[8-11]: b   reserved
	//
	// godbus decodes structs as []interface{}.

	// We need at least the state name (position 5) and value (position 6).
	if len(sig.Body) < 7 {
		return
	}

	stateName, ok := sig.Body[5].(string)
	if !ok || stateName != "focused" {
		return
	}

	stateValue, ok := sig.Body[6].(int32)
	if !ok {
		return
	}

	// Mark as healthy — AT-SPI2 is delivering events.
	w.healthy = true

	if stateValue != 1 {
		// Unfocused event — we only need focused events.
		// The next focused event will tell us the new widget's role.
		return
	}

	// Extract source object reference: Body[1] is (so) decoded as []interface{}.
	sourceObj, ok := sig.Body[1].([]interface{})
	if !ok || len(sourceObj) < 2 {
		return
	}
	busName, _ := sourceObj[0].(string)
	objPath, _ := sourceObj[1].(string)
	if busName == "" || objPath == "" {
		return
	}

	// Query the accessible object's role.
	var role uint32
	err := w.conn.Object(busName, dbus.ObjectPath(objPath)).
		Call("org.a11y.atspi.Accessible.GetRole", 0).Store(&role)
	if err != nil {
		log.Printf("[ATSPI2] GetRole failed for %s %s: %v", busName, objPath, err)
		return
	}

	isText := textRoles[role]
	log.Printf("[ATSPI2] focus: bus=%s path=%s role=%d isText=%v", busName, objPath, role, isText)
	w.bus.Publish(bus.TopicTextInputFocus, isText)
}
