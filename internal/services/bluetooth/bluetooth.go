package bluetooth

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/dbusutil"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const (
	bluezService = "org.bluez"
	bluezAdapter = "/org/bluez/hci0"
	bluezIface   = "org.bluez.Adapter1"
)

type Service struct {
	conn dbusutil.DBusConn
	bus  *bus.Bus
}

func New(conn *dbus.Conn, b *bus.Bus) *Service {
	return &Service{conn: dbusutil.NewRealConn(conn), bus: b}
}

func NewWithConn(conn dbusutil.DBusConn, b *bus.Bus) *Service {
	return &Service{conn: conn, bus: b}
}

func (s *Service) Run(ctx context.Context) error {
	// Emit initial state (best-effort; adapter may be absent).
	_ = s.poll()

	signals := make(chan *dbus.Signal, 8)
	s.conn.Signal(signals)
	_ = s.conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchObjectPath(bluezAdapter),
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-signals:
			if !ok {
				return nil
			}
			_ = s.poll()
		}
	}
}

func (s *Service) poll() error {
	obj := s.conn.Object(bluezService, bluezAdapter)
	poweredV, err := obj.GetProperty(bluezIface + ".Powered")
	if err != nil {
		return fmt.Errorf("bluetooth poll: %w", err)
	}
	powered, _ := poweredV.Value().(bool)
	s.bus.Publish(bus.TopicBluetooth, state.BluetoothState{
		Powered: powered,
	})
	return nil
}

// SetPowered enables or disables the Bluetooth adapter.
func (s *Service) SetPowered(enabled bool) error {
	obj := s.conn.Object(bluezService, bluezAdapter)
	return obj.SetProperty(bluezIface+".Powered", dbus.MakeVariant(enabled))
}

// StartScan requests a Bluetooth device discovery scan.
func (s *Service) StartScan() error {
	obj := s.conn.Object(bluezService, bluezAdapter)
	return obj.Call(bluezIface+".StartDiscovery", 0).Err
}

// StopScan stops an ongoing Bluetooth discovery scan.
func (s *Service) StopScan() error {
	obj := s.conn.Object(bluezService, bluezAdapter)
	return obj.Call(bluezIface+".StopDiscovery", 0).Err
}

// GetDevices returns all known Bluetooth devices.
func (s *Service) GetDevices() ([]state.BluetoothDevice, error) {
	managed := s.conn.Object(bluezService, "/")
	var result map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err := managed.Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&result)
	if err != nil {
		return nil, err
	}

	var devices []state.BluetoothDevice
	for path, ifaces := range result {
		if _, ok := ifaces["org.bluez.Device1"]; !ok {
			continue
		}
		devObj := s.conn.Object(bluezService, path)

		name := ""
		if v, err := devObj.GetProperty("org.bluez.Device1.Name"); err == nil {
			name, _ = v.Value().(string)
		}
		paired := false
		if v, err := devObj.GetProperty("org.bluez.Device1.Paired"); err == nil {
			paired, _ = v.Value().(bool)
		}
		connected := false
		if v, err := devObj.GetProperty("org.bluez.Device1.Connected"); err == nil {
			connected, _ = v.Value().(bool)
		}
		icon := "bluetooth"
		if v, err := devObj.GetProperty("org.bluez.Device1.Icon"); err == nil {
			icon, _ = v.Value().(string)
		}

		devices = append(devices, state.BluetoothDevice{
			Address:   string(path),
			Name:      name,
			Paired:    paired,
			Connected: connected,
			Icon:      icon,
		})
	}

	s.bus.Publish(bus.TopicBluetoothDevices, devices)
	return devices, nil
}

// PairDevice initiates pairing with a Bluetooth device.
func (s *Service) PairDevice(addr string) error {
	obj := s.conn.Object(bluezService, dbus.ObjectPath(addr))
	return obj.Call("org.bluez.Device1.Pair", 0).Err
}

// ConnectDevice connects to an already-paired Bluetooth device.
func (s *Service) ConnectDevice(addr string) error {
	obj := s.conn.Object(bluezService, dbus.ObjectPath(addr))
	return obj.Call("org.bluez.Device1.Connect", 0).Err
}

// DisconnectDevice disconnects a Bluetooth device.
func (s *Service) DisconnectDevice(addr string) error {
	obj := s.conn.Object(bluezService, dbus.ObjectPath(addr))
	return obj.Call("org.bluez.Device1.Disconnect", 0).Err
}
