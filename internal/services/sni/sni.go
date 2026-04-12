package sni

import (
	"context"
	"log"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/puzpuzpuz/xsync/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/dbusutil"
)

const (
	watcherDest  = "org.kde.StatusNotifierWatcher"
	watcherPath  = "/StatusNotifierWatcher"
	watcherIface = "org.kde.StatusNotifierWatcher"
	itemIface    = "org.kde.StatusNotifierItem"
	hostIface    = "org.kde.StatusNotifierHost"
)

type TrayItem struct {
	BusName  string
	Path     dbus.ObjectPath
	Title    string
	IconName string
	Status   string
	ID       string
}

func (t *TrayItem) Key() string { return t.BusName + string(t.Path) }

type Service struct {
	conn  dbusutil.DBusConn
	bus   *bus.Bus
	items *xsync.Map[string, *TrayItem]
}

func New(conn dbusutil.DBusConn, b *bus.Bus) *Service {
	return &Service{
		conn:  conn,
		bus:   b,
		items: xsync.NewMap[string, *TrayItem](),
	}
}

func NewWithDefaults(b *bus.Bus) *Service {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return &Service{bus: b, items: xsync.NewMap[string, *TrayItem]()}
	}
	return &Service{conn: dbusutil.NewRealConn(conn), bus: b, items: xsync.NewMap[string, *TrayItem]()}
}

// Run starts watching for tray item signals. Blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	// Register as a StatusNotifierHost.
	hostObj := s.conn.Object(watcherDest, watcherPath)
	hostObj.Call(watcherIface+".RegisterStatusNotifierHost", 0, dbus.ObjectPath("/org/freedesktop/Notifications"))

	// Listen for item registered/unregistered.
	ch := make(chan *dbus.Signal, 16)
	s.conn.Signal(ch)
	defer s.conn.RemoveSignal(ch)
	if err := s.conn.AddMatchSignal(
		dbus.WithMatchInterface(watcherIface),
		dbus.WithMatchMember("StatusNotifierItemRegistered"),
	); err != nil {
		log.Printf("[sni] AddMatchSignal: %v", err)
	}
	if err := s.conn.AddMatchSignal(
		dbus.WithMatchInterface(watcherIface),
		dbus.WithMatchMember("StatusNotifierItemUnregistered"),
	); err != nil {
		log.Printf("[sni] AddMatchSignal: %v", err)
	}

	// Fetch initial items.
	s.fetchRegisteredItems()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig, ok := <-ch:
			if !ok {
				return nil
			}
			if sig.Path != watcherPath {
				continue
			}
			if len(sig.Body) < 1 {
				continue
			}
			servicePath, ok := sig.Body[0].(string)
			if !ok {
				continue
			}

			switch sig.Name {
			case watcherIface + ".StatusNotifierItemRegistered":
				s.addItem(servicePath)
			case watcherIface + ".StatusNotifierItemUnregistered":
				s.removeItem(servicePath)
			}
		}
	}
}

func (s *Service) fetchRegisteredItems() {
	obj := s.conn.Object(watcherDest, watcherPath)
	v, err := obj.GetProperty(watcherIface + ".RegisteredStatusNotifierItems")
	if err != nil {
		return
	}
	items, ok := v.Value().([]string)
	if !ok {
		return
	}
	for _, itemPath := range items {
		s.addItem(itemPath)
	}
}

// addItem parses the service path and queries item properties.
func (s *Service) addItem(servicePath string) {
	busName, objPath := parseServicePath(servicePath)
	key := busName + string(objPath)

	if _, exists := s.items.Load(key); exists {
		return
	}

	item := &TrayItem{
		BusName: busName,
		Path:    objPath,
	}
	s.queryItemProps(item)

	s.items.Store(key, item)
	s.bus.Publish(bus.TopicTrayItems, s.AllItems())
}

func (s *Service) removeItem(servicePath string) {
	busName, objPath := parseServicePath(servicePath)
	key := busName + string(objPath)

	s.items.Delete(key)
	s.bus.Publish(bus.TopicTrayItems, s.AllItems())
}

func (s *Service) queryItemProps(item *TrayItem) {
	obj := s.conn.Object(item.BusName, item.Path)

	if v, err := obj.GetProperty(itemIface + ".Title"); err == nil {
		item.Title, _ = v.Value().(string)
	}
	if v, err := obj.GetProperty(itemIface + ".IconName"); err == nil {
		item.IconName, _ = v.Value().(string)
	}
	if v, err := obj.GetProperty(itemIface + ".Status"); err == nil {
		item.Status, _ = v.Value().(string)
	}
	if v, err := obj.GetProperty(itemIface + ".Id"); err == nil {
		item.ID, _ = v.Value().(string)
	}
}

// AllItems returns a snapshot of current tray items.
func (s *Service) AllItems() []*TrayItem {
	items := make([]*TrayItem, 0)
	s.items.Range(func(_ string, item *TrayItem) bool {
		items = append(items, item)
		return true
	})
	return items
}

// Activate sends the Activate message to the tray item.
func (s *Service) Activate(key string) {
	item, ok := s.items.Load(key)
	if !ok {
		return
	}
	s.conn.Object(item.BusName, item.Path).Call(itemIface+".Activate", 0, 0, 0)
}

// parseServicePath splits "busName" or "busName/objectPath" into components.
func parseServicePath(servicePath string) (busName string, objPath dbus.ObjectPath) {
	if len(servicePath) > 1 && servicePath[0] == '/' {
		// Just an object path — use the watcher's bus name.
		return watcherDest, dbus.ObjectPath(servicePath)
	}
	// Try to split busName/objectPath.
	if idx := strings.Index(servicePath, "/"); idx > 0 {
		return servicePath[:idx], dbus.ObjectPath(servicePath[idx:])
	}
	return servicePath, dbus.ObjectPath("/StatusNotifierItem")
}
