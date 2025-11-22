package network

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const (
	nmDest            = "org.freedesktop.NetworkManager"
	nmPath            = "/org/freedesktop/NetworkManager"
	nmIface           = "org.freedesktop.NetworkManager"
	nmDeviceWireless  = "org.freedesktop.NetworkManager.Device.Wireless"
	nmAccessPoint     = "org.freedesktop.NetworkManager.AccessPoint"
	nmPropertiesIface = "org.freedesktop.DBus.Properties"
)

// NMState values from NetworkManager spec.
const (
	nmStateConnectedGlobal = uint32(70)
	nmStateConnectedSite   = uint32(60)
	nmStateConnectedLocal  = uint32(50)
)

// DBusObjecter can retrieve a DBus object by destination and path.
type DBusObjecter interface {
	Object(dest string, path dbus.ObjectPath) dbus.BusObject
	Signal(ch chan<- *dbus.Signal)
	BusObject() dbus.BusObject
}

type realConn struct{ conn *dbus.Conn }

func (r *realConn) Object(dest string, path dbus.ObjectPath) dbus.BusObject {
	return r.conn.Object(dest, path)
}
func (r *realConn) Signal(ch chan<- *dbus.Signal) { r.conn.Signal(ch) }
func (r *realConn) BusObject() dbus.BusObject     { return r.conn.BusObject() }

// Service watches NetworkManager for connectivity and SSID changes.
type Service struct {
	conn DBusObjecter
	bus  *bus.Bus
}

func New(conn *dbus.Conn, b *bus.Bus) *Service {
	return &Service{conn: &realConn{conn: conn}, bus: b}
}

func NewWithConn(conn DBusObjecter, b *bus.Bus) *Service {
	return &Service{conn: conn, bus: b}
}

func (s *Service) Run(ctx context.Context) error {
	ch := make(chan *dbus.Signal, 16)
	s.conn.Signal(ch)

	s.conn.BusObject().Call(
		"org.freedesktop.DBus.AddMatch", 0,
		fmt.Sprintf("type='signal',sender='%s',interface='%s',member='PropertiesChanged'",
			nmDest, nmPropertiesIface),
	)

	// Emit initial state.
	s.query()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-ch:
			if !ok {
				return nil
			}
			s.query()
		}
	}
}

func (s *Service) query() {
	ns, err := s.fetchState()
	if err != nil {
		return
	}
	s.bus.Publish(bus.TopicNetwork, ns)
}

func (s *Service) fetchState() (state.NetworkState, error) {
	nmObj := s.conn.Object(nmDest, nmPath)

	connStateV, err := nmObj.GetProperty(nmIface + ".Connectivity")
	if err != nil {
		return state.NetworkState{}, fmt.Errorf("get connectivity: %w", err)
	}
	connState, _ := connStateV.Value().(uint32)
	connected := connState >= nmStateConnectedLocal

	// Try to get SSID from the primary WiFi device.
	ssid := ""
	devicesV, err := nmObj.GetProperty(nmIface + ".Devices")
	if err == nil {
		if paths, ok := devicesV.Value().([]dbus.ObjectPath); ok {
			for _, p := range paths {
				if s2, ok := s.getWifiSSID(p); ok {
					ssid = s2
					break
				}
			}
		}
	}

	// Get wireless enabled state.
	wirelessV, err := nmObj.GetProperty(nmIface + ".WirelessEnabled")
	if err == nil {
		wirelessEnabled, _ := wirelessV.Value().(bool)
		return state.NetworkState{Connected: connected, SSID: ssid, WirelessEnabled: wirelessEnabled}, nil
	}

	return state.NetworkState{Connected: connected, SSID: ssid}, nil
}

// SetWiFi enables or disables the WiFi adapter.
func (s *Service) SetWiFi(enabled bool) error {
	nmObj := s.conn.Object(nmDest, nmPath)
	return nmObj.SetProperty(nmIface+".WirelessEnabled", dbus.MakeVariant(enabled))
}

func (s *Service) getWifiSSID(devicePath dbus.ObjectPath) (string, bool) {
	devObj := s.conn.Object(nmDest, devicePath)
	apPathV, err := devObj.GetProperty(nmDeviceWireless + ".ActiveAccessPoint")
	if err != nil {
		return "", false
	}
	apPath, ok := apPathV.Value().(dbus.ObjectPath)
	if !ok || apPath == "/" {
		return "", false
	}
	apObj := s.conn.Object(nmDest, apPath)
	ssidV, err := apObj.GetProperty(nmAccessPoint + ".Ssid")
	if err != nil {
		return "", false
	}
	ssidBytes, ok := ssidV.Value().([]byte)
	if !ok {
		return "", false
	}
	return string(ssidBytes), true
}

// ScanWiFi requests a WiFi scan and returns available networks.
func (s *Service) ScanWiFi() ([]state.WiFiNetwork, error) {
	devicesV, err := s.conn.Object(nmDest, nmPath).GetProperty(nmIface + ".Devices")
	if err != nil {
		return nil, err
	}
	paths, ok := devicesV.Value().([]dbus.ObjectPath)
	if !ok {
		return nil, nil
	}

	var networks []state.WiFiNetwork
	seen := make(map[string]bool)

	// Get currently connected SSID.
	currentSSID := ""
	if ns, err := s.fetchState(); err == nil {
		currentSSID = ns.SSID
	}

	for _, p := range paths {
		devObj := s.conn.Object(nmDest, p)
		// Request scan.
		devObj.Call(nmDeviceWireless+".RequestScan", 0)

		// Get all access points.
		apsV, err := devObj.GetProperty(nmDeviceWireless + ".AllAccessPoints")
		if err != nil {
			continue
		}
		apPaths, ok := apsV.Value().([]dbus.ObjectPath)
		if !ok {
			continue
		}

		for _, apPath := range apPaths {
			apObj := s.conn.Object(nmDest, apPath)
			ssidV, err := apObj.GetProperty(nmAccessPoint + ".Ssid")
			if err != nil {
				continue
			}
			ssidBytes, ok := ssidV.Value().([]byte)
			if !ok {
				continue
			}
			ssid := string(ssidBytes)
			if ssid == "" || seen[ssid] {
				continue
			}
			seen[ssid] = true

			strength := 0
			if strV, err := apObj.GetProperty(nmAccessPoint + ".Strength"); err == nil {
				if v, ok := strV.Value().(uint8); ok {
					strength = int(v)
				}
			}

			security := ""
			if flagsV, err := apObj.GetProperty(nmAccessPoint + ".WpaFlags"); err == nil {
				flags, _ := flagsV.Value().(uint32)
				if flags > 0 {
					security = "WPA"
				}
			}
			if rsnV, err := apObj.GetProperty(nmAccessPoint + ".RsnFlags"); err == nil {
				flags, _ := rsnV.Value().(uint32)
				if flags > 0 {
					security = "WPA2"
				}
			}

			networks = append(networks, state.WiFiNetwork{
				SSID:       ssid,
				Signal:     int(strength),
				Security:   security,
				Connected: ssid == currentSSID,
			})
		}
	}

	s.bus.Publish(bus.TopicWiFiNetworks, networks)
	return networks, nil
}

// ConnectWiFi activates a connection for the given SSID.
func (s *Service) ConnectWiFi(ssid string) error {
	connsV, err := s.conn.Object(nmDest, nmPath).GetProperty(nmIface + ".Connections")
	if err != nil {
		return err
	}
	connPaths, ok := connsV.Value().([]dbus.ObjectPath)
	if !ok {
		return fmt.Errorf("no connections found")
	}

	for _, cp := range connPaths {
		connObj := s.conn.Object(nmDest, cp)
		sidV, err := connObj.GetProperty("org.freedesktop.NetworkManager.Connection.Settings")
		if err != nil {
			continue
		}
		settings, ok := sidV.Value().(map[string]map[string]dbus.Variant)
		if !ok {
			continue
		}
		if wireless, ok := settings["802-11-wireless"]; ok {
			if sv, ok := wireless["ssid"]; ok {
				if connSSID, ok := sv.Value().(string); ok && connSSID == ssid {
					s.conn.Object(nmDest, nmPath).Call(nmIface+".ActivateConnection", 0, cp, dbus.ObjectPath("/"))
					return nil
				}
			}
		}
	}

	return fmt.Errorf("no existing connection for SSID %q", ssid)
}
