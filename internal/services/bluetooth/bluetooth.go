package bluetooth

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

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
	mu   sync.Mutex // serializes poll() — godbus BusObject is not thread-safe
}

func New(conn dbusutil.DBusConn, b *bus.Bus) *Service {
	return &Service{conn: conn, bus: b}
}

func NewWithDefaults(b *bus.Bus) *Service {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return &Service{bus: b}
	}
	return &Service{conn: dbusutil.NewRealConn(conn), bus: b}
}

func (s *Service) Run(ctx context.Context) error {
	if s.conn == nil {
		<-ctx.Done()
		return ctx.Err()
	}

	// Emit initial state (best-effort; adapter may be absent).
	_ = s.poll()

	signals := make(chan *dbus.Signal, 8)
	s.conn.Signal(signals)
	if err := s.conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		log.Printf("[bluetooth] AddMatchSignal: %v", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig, ok := <-signals:
			if !ok {
				return nil
			}
			if sig.Path != bluezAdapter && !isBlueZDevicePath(sig.Path) {
				continue
			}
			log.Printf("[bluetooth] Run: received D-Bus signal: %v", sig)
			_ = s.poll()
		}
	}
}

func (s *Service) poll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	obj := s.conn.Object(bluezService, bluezAdapter)
	poweredV, err := obj.GetProperty(bluezIface + ".Powered")
	if err != nil {
		log.Printf("[bluetooth] poll GetProperty error: %v", err)
		return fmt.Errorf("bluetooth poll: %w", err)
	}
	powered, _ := poweredV.Value().(bool)

	// BlueZ transitions through PowerState (string) before updating Powered
	// (bool). Check PowerState so we don't publish stale Powered=true while
	// the adapter is disabling.
	if psV, err := obj.GetProperty(bluezIface + ".PowerState"); err == nil {
		if ps, ok := psV.Value().(string); ok {
			log.Printf("[bluetooth] poll: Powered=%v PowerState=%s", powered, ps)
			switch ps {
			case "off", "on-disabling":
				powered = false
			}
		}
	} else {
		log.Printf("[bluetooth] poll: Powered=%v", powered)
	}

	bs := state.BluetoothState{Powered: powered}
	if powered {
		devices, err := s.GetDevices()
		if err == nil {
			for _, d := range devices {
				if d.Connected {
					bs.Connected = true
					bs.DeviceName = d.Name
					break
				}
			}
		}
	}
	s.bus.Publish(bus.TopicBluetooth, bs)
	return nil
}

// SetPowered enables or disables the Bluetooth adapter.
// If powering off and BlueZ is busy (discovery running), it stops discovery first.
// On error, re-polls to publish the actual state so the UI reverts correctly.
func (s *Service) SetPowered(enabled bool) error {
	log.Printf("[bluetooth] SetPowered(%v) called", enabled)
	obj := s.conn.Object(bluezService, bluezAdapter)
	err := obj.SetProperty(bluezIface+".Powered", dbus.MakeVariant(enabled))
	if err != nil && !enabled {
		// BlueZ returns "Busy" if discovery is running; stop it and retry.
		log.Printf("[bluetooth] SetPowered(false) failed (%v), stopping discovery and retrying", err)
		_ = obj.Call(bluezIface+".StopDiscovery", 0).Err
		err = obj.SetProperty(bluezIface+".Powered", dbus.MakeVariant(false))
	}
	if err != nil {
		log.Printf("[bluetooth] SetPowered(%v) failed: %v — re-polling to publish actual state", enabled, err)
		_ = s.poll()
		return err
	}
	log.Printf("[bluetooth] SetPowered(%v) succeeded", enabled)
	return nil
}

// StartScan requests a Bluetooth device discovery scan.
// If discovery is already in progress, it stops and restarts.
func (s *Service) StartScan() error {
	log.Printf("[bluetooth] StartScan called")
	obj := s.conn.Object(bluezService, bluezAdapter)
	err := obj.Call(bluezIface+".StartDiscovery", 0).Err
	if err != nil {
		log.Printf("[bluetooth] StartScan failed (%v), stopping and retrying", err)
		_ = obj.Call(bluezIface+".StopDiscovery", 0).Err
		err = obj.Call(bluezIface+".StartDiscovery", 0).Err
	}
	if err != nil {
		log.Printf("[bluetooth] StartScan error: %v", err)
	} else {
		log.Printf("[bluetooth] StartScan succeeded")
	}
	return err
}

// GetDevices returns all known Bluetooth devices.
func (s *Service) GetDevices() ([]state.BluetoothDevice, error) {
	managed := s.conn.Object(bluezService, "/")
	var result map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err := managed.Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&result)
	if err != nil {
		log.Printf("[bluetooth] GetDevices GetManagedObjects error: %v", err)
		return nil, err
	}
	log.Printf("[bluetooth] GetDevices: %d managed objects", len(result))

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
		trusted := false
		if v, err := devObj.GetProperty("org.bluez.Device1.Trusted"); err == nil {
			trusted, _ = v.Value().(bool)
		}

		devices = append(devices, state.BluetoothDevice{
			Address:   string(path),
			Name:      name,
			Paired:    paired,
			Connected: connected,
			Icon:      icon,
			Trusted:   trusted,
		})
	}

	s.bus.Publish(bus.TopicBluetoothDevices, devices)
		log.Printf("[bluetooth] GetDevices: published %d devices", len(devices))
	return devices, nil
}

// PairDevice initiates pairing with a Bluetooth device and auto-trusts it.
func (s *Service) PairDevice(addr string) error {
	obj := s.conn.Object(bluezService, dbus.ObjectPath(addr))
	if err := obj.Call("org.bluez.Device1.Pair", 0).Err; err != nil {
		return err
	}
	if err := s.SetTrusted(addr, true); err != nil { log.Printf("[bluetooth] SetTrusted after pair: %v", err) }
	return nil
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

// SetTrusted sets the trusted status of a Bluetooth device.
func (s *Service) SetTrusted(devicePath string, trusted bool) error {
	obj := s.conn.Object(bluezService, dbus.ObjectPath(devicePath))
	return obj.SetProperty("org.bluez.Device1.Trusted", dbus.MakeVariant(trusted))
}

func isBlueZDevicePath(path dbus.ObjectPath) bool {
	return strings.HasPrefix(string(path), "/org/bluez/hci0/dev_")
}
