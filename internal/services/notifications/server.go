package notifications

import (
	"fmt"
	"sync/atomic"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const (
	dbusName  = "org.freedesktop.Notifications"
	dbusPath  = "/org/freedesktop/Notifications"
	dbusIface = "org.freedesktop.Notifications"
)

// Server implements the org.freedesktop.Notifications DBus interface.
type Server struct {
	pub bus.Publisher
	id  atomic.Uint32
}

func New(pub bus.Publisher) *Server {
	return &Server{pub: pub}
}

// Notify handles an incoming notification and publishes it on the bus.
func (s *Server) Notify(
	appName string,
	replacesID uint32,
	appIcon string,
	summary string,
	body string,
	actions []string,
	hints map[string]dbus.Variant,
	expireTimeout int32,
) (uint32, *dbus.Error) {
	var id uint32
	if replacesID != 0 {
		id = replacesID
	} else {
		id = s.id.Add(1)
	}

	urgency := byte(1)
	if u, ok := hints["urgency"]; ok {
		if v, ok := u.Value().(byte); ok {
			urgency = v
		}
	}

	s.pub.Publish(bus.TopicNotification, state.Notification{
		ID:      id,
		AppName: appName,
		Summary: summary,
		Body:    body,
		Urgency: urgency,
		Timeout: expireTimeout,
	})
	return id, nil
}

func (s *Server) CloseNotification(id uint32) *dbus.Error {
	return nil
}

func (s *Server) GetCapabilities() ([]string, *dbus.Error) {
	return []string{"body", "actions", "urgency", "body-markup"}, nil
}

func (s *Server) GetServerInformation() (name, vendor, version, specVersion string, err *dbus.Error) {
	return "snry-shell", "snry", "0.1.0", "1.2", nil
}

// Register exports the server on the session bus and acquires the well-known name.
func Register(conn *dbus.Conn, srv *Server) error {
	if err := conn.Export(srv, dbusPath, dbusIface); err != nil {
		return fmt.Errorf("export notifications: %w", err)
	}
	reply, err := conn.RequestName(dbusName, dbus.NameFlagReplaceExisting)
	if err != nil {
		return fmt.Errorf("request dbus name: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("could not own %s: reply=%d", dbusName, reply)
	}
	return nil
}
