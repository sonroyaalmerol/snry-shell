// Package sni implements a StatusNotifierItem host that watches for system
// tray items via DBus and publishes their state on the event bus.
package sni

import (
	"context"
	"log"
	"path"
	"sync"

	"github.com/godbus/dbus/v5"
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

type Service struct {
	mu    sync.RWMutex
	conn  dbusutil.DBusConn
	bus   *bus.Bus
	items map[string]*TrayItem
}

func New(conn *dbus.Conn, b *bus.Bus) *Service {
	return &Service{
		conn:  dbusutil.NewRealConn(conn),
		bus:   b,
		items: make(map[string]*TrayItem),
	}
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
		log.Printf("[SNI] AddMatchSignal: %v", err)
	}
	if err := s.conn.AddMatchSignal(
		dbus.WithMatchInterface(watcherIface),
		dbus.WithMatchMember("StatusNotifierItemUnregistered"),
	); err != nil {
		log.Printf("[SNI] AddMatchSignal: %v", err)
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
// The path format is either "busName" or "busName/objectPath".
func (s *Service) addItem(servicePath string) {
	busName, objPath := parseServicePath(servicePath)
	key := busName + string(objPath)

	s.mu.Lock()
	if _, exists := s.items[key]; exists {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	item := &TrayItem{
		BusName: busName,
		Path:    objPath,
	}
	s.queryItemProps(item)

	s.mu.Lock()
	s.items[key] = item
	s.mu.Unlock()

	s.bus.Publish(bus.TopicTrayItems, s.AllItems())
}

func (s *Service) removeItem(servicePath string) {
	busName, objPath := parseServicePath(servicePath)
	key := busName + string(objPath)

	s.mu.Lock()
	delete(s.items, key)
	s.mu.Unlock()

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
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]*TrayItem, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	return items
}

// Activate sends the Activate message to the tray item.
func (s *Service) Activate(key string) {
	s.mu.RLock()
	item, ok := s.items[key]
	s.mu.RUnlock()
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
	busName, objPathStr := path.Split(servicePath)
	if objPathStr != "" {
		return busName, dbus.ObjectPath("/" + objPathStr)
	}
	return servicePath, dbus.ObjectPath("/StatusNotifierItem")
}
