// Package tabletmode monitors systemd-logind for tablet mode changes and
// publishes them on the bus. Uses the system D-Bus to subscribe to
// PropertiesChanged signals on the logind session object.
package tabletmode

import (
	"context"
	"log"
	"os"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/dbusutil"
)

const (
	logindDest   = "org.freedesktop.login1"
	logindMgrPath = "/org/freedesktop/login1"
	logindIface  = "org.freedesktop.login1.Session"
)

// Service monitors logind for tablet mode changes.
type Service struct {
	conn    dbusutil.DBusConn
	bus     *bus.Bus
	session string // D-Bus object path for the current session
}

func New(conn *dbus.Conn, b *bus.Bus) *Service {
	return &Service{conn: dbusutil.NewRealConn(conn), bus: b}
}

// Run resolves the current session, queries the initial state, and
// subscribes to PropertiesChanged signals. Blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	// Find the current session path.
	session, err := s.resolveSession()
	if err != nil {
		log.Printf("[TABLETMODE] cannot resolve session: %v", err)
		return nil
	}
	s.session = session

	ch := make(chan *dbus.Signal, 16)
	s.conn.Signal(ch)

	// Subscribe to PropertiesChanged on the session object.
	if err := s.conn.AddMatchSignal(dbus.WithMatchObjectPath(dbus.ObjectPath(session))); err != nil {
		log.Printf("[TABLETMODE] AddMatchSignal: %v", err)
		return nil
	}

	// Query initial state.
	s.query()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig, ok := <-ch:
			if !ok {
				return nil
			}
			// Only react to signals on our session object.
			if sig.Path != dbus.ObjectPath(session) {
				continue
			}
			s.query()
		}
	}
}

// resolveSession finds the current user's logind session object path
// via org.freedesktop.login1.Manager.GetSessionByPID or XDG_SESSION_ID.
func (s *Service) resolveSession() (string, error) {
	// Try XDG_SESSION_ID first.
	if id := os.Getenv("XDG_SESSION_ID"); id != "" {
		path := logindMgrPath + "/session_" + id
		if s.conn.Object(logindDest, dbus.ObjectPath(path)) != nil {
			return path, nil
		}
	}

	// Fall back to GetSessionByPID (auto selects the active session).
	mgr := s.conn.Object(logindDest, dbus.ObjectPath(logindMgrPath))
	var sessionPath dbus.ObjectPath
	err := mgr.Call("org.freedesktop.login1.Manager.GetSessionByPID", 0, uint32(os.Getpid())).Store(&sessionPath)
	if err != nil {
		return "", err
	}
	return string(sessionPath), nil
}

// query reads the current TabletMode property from logind and publishes it.
func (s *Service) query() {
	mode := s.fetchTabletMode()
	s.bus.Publish(bus.TopicTabletMode, mode)
}

// fetchTabletMode reads the TabletMode property from logind.
// Returns: "enabled" (tablet), "disabled" (laptop), or "indeterminate".
func (s *Service) fetchTabletMode() string {
	obj := s.conn.Object(logindDest, dbus.ObjectPath(s.session))
	v, err := obj.GetProperty(logindIface + ".TabletMode")
	if err != nil {
		return "indeterminate"
	}
	mode, ok := v.Value().(string)
	if !ok {
		return "indeterminate"
	}
	return mode
}

// IsEnabled is a convenience for subscribers to check the mode.
func IsEnabled(mode string) bool {
	return mode == "enabled"
}
