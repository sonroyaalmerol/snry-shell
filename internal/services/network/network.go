package network

import (
	"context"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/dbusutil"
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

const (
	nmDeviceTypeWifi = uint32(2)

	nmStateConnectedGlobal = uint32(70)
	nmStateConnectedSite   = uint32(60)
	nmStateConnectedLocal  = uint32(50)
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
	ch := make(chan *dbus.Signal, 16)
	s.conn.Signal(ch)

	busObj := s.conn.BusObject()
	if busObj == nil {
		return fmt.Errorf("no D-Bus connection")
	}
	busObj.Call(
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
	if nmObj == nil {
		return state.NetworkState{}, fmt.Errorf("no D-Bus connection")
	}

	connStateV, err := nmObj.GetProperty(nmIface + ".Connectivity")
	if err != nil {
		return state.NetworkState{}, fmt.Errorf("get connectivity: %w", err)
	}
	connState, _ := connStateV.Value().(uint32)
	connected := connState >= nmStateConnectedLocal

	// Try to get SSID from the primary WiFi device.
	ssid := ""
	if wifiPaths, err := s.wifiDevicePaths(); err == nil {
		for _, p := range wifiPaths {
			if s2, ok := s.getWifiSSID(p); ok {
				ssid = s2
				break
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
	if nmObj == nil {
		return fmt.Errorf("no D-Bus connection")
	}
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

// wifiDevicePaths returns only WiFi device paths from NetworkManager.
func (s *Service) wifiDevicePaths() ([]dbus.ObjectPath, error) {
	nmObj := s.conn.Object(nmDest, nmPath)
	if nmObj == nil {
		return nil, fmt.Errorf("no D-Bus connection")
	}
	devicesV, err := nmObj.GetProperty(nmIface + ".Devices")
	if err != nil {
		return nil, err
	}
	paths, ok := devicesV.Value().([]dbus.ObjectPath)
	if !ok {
		return nil, nil
	}
	var wifiPaths []dbus.ObjectPath
	for _, p := range paths {
		devObj := s.conn.Object(nmDest, p)
		dtV, err := devObj.GetProperty(nmIface + ".Device.DeviceType")
		if err != nil {
			continue
		}
		dt, _ := dtV.Value().(uint32)
		if dt == nmDeviceTypeWifi {
			wifiPaths = append(wifiPaths, p)
		}
	}
	return wifiPaths, nil
}

// ScanWiFi requests a WiFi scan and returns available networks.
// It always publishes to TopicWiFiNetworks so the widget can update.
func (s *Service) ScanWiFi() ([]state.WiFiNetwork, error) {
	networks := []state.WiFiNetwork{}
	defer func() { s.bus.Publish(bus.TopicWiFiNetworks, networks) }()

	paths, err := s.wifiDevicePaths()
	if err != nil {
		return networks, err
	}

	seen := make(map[string]bool)

	// Get currently connected SSID.
	currentSSID := ""
	if ns, err := s.fetchState(); err == nil {
		currentSSID = ns.SSID
	}

	// Request scan on all WiFi devices, then wait for NM to discover APs.
	for _, p := range paths {
		devObj := s.conn.Object(nmDest, p)
		devObj.Call(nmDeviceWireless+".RequestScan", 0)
	}
	time.Sleep(3 * time.Second)

	for _, p := range paths {
		devObj := s.conn.Object(nmDest, p)

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
				SSID:      ssid,
				Signal:    int(strength),
				Security:  security,
				Connected: ssid == currentSSID,
			})
		}
	}

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
