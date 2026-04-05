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

	stateV, err := nmObj.GetProperty(nmIface + ".State")
	if err != nil {
		return state.NetworkState{}, fmt.Errorf("get state: %w", err)
	}
	nmState, _ := stateV.Value().(uint32)
	connected := nmState >= nmStateConnectedLocal

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

	// Build saved SSID set.
	savedSet := make(map[string]bool)
	if saved, err := s.GetSavedSSIDs(); err == nil {
		for _, s := range saved {
			savedSet[s] = true
		}
	}

	paths, err := s.wifiDevicePaths()
	if err != nil {
		return networks, err
	}

	seen := make(map[string]bool)

	// Request scan on all WiFi devices, then wait for NM to discover APs.
	for _, p := range paths {
		devObj := s.conn.Object(nmDest, p)
		devObj.Call(nmDeviceWireless+".RequestScan", 0)
	}
	time.Sleep(3 * time.Second)

	// Get currently connected SSID after scan wait so connection
	// changes (e.g. switching networks) are reflected.
	currentSSID := ""
	if ns, err := s.fetchState(); err == nil {
		currentSSID = ns.SSID
	}

	for _, p := range paths {
		devObj := s.conn.Object(nmDest, p)

		apsV, err := devObj.GetProperty(nmDeviceWireless + ".AccessPoints")
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
				Saved:     savedSet[ssid],
			})
		}
	}

	return networks, nil
}

// ConnectWiFi activates a saved connection for the given SSID.
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
		var settings map[string]map[string]dbus.Variant
		if err := connObj.Call("org.freedesktop.NetworkManager.Settings.Connection.GetSettings", 0).Store(&settings); err != nil {
			continue
		}
		if wireless, ok := settings["802-11-wireless"]; ok {
			if sv, ok := wireless["ssid"]; ok {
				if ssidBytes, ok := sv.Value().([]byte); ok && string(ssidBytes) == ssid {
					s.conn.Object(nmDest, nmPath).Call(nmIface+".ActivateConnection", 0, cp, dbus.ObjectPath("/"))
					return nil
				}
			}
		}
	}

	return fmt.Errorf("no existing connection for SSID %q", ssid)
}

// DisconnectWiFi deactivates the current WiFi connection.
func (s *Service) DisconnectWiFi() error {
	nmObj := s.conn.Object(nmDest, nmPath)
	if nmObj == nil {
		return fmt.Errorf("no D-Bus connection")
	}
	primaryV, err := nmObj.GetProperty(nmIface + ".PrimaryConnection")
	if err != nil {
		return fmt.Errorf("get primary connection: %w", err)
	}
	primaryPath, ok := primaryV.Value().(dbus.ObjectPath)
	if !ok || primaryPath == "/" {
		return fmt.Errorf("no active connection")
	}
	return nmObj.Call(nmIface+".DeactivateConnection", 0, primaryPath).Err
}

// ForgetWiFi removes the saved connection profile for the given SSID.
func (s *Service) ForgetWiFi(ssid string) error {
	settingsObj := s.conn.Object(nmDest, "/org/freedesktop/NetworkManager/Settings")
	connsV, err := settingsObj.GetProperty("org.freedesktop.NetworkManager.Settings.Connections")
	if err != nil {
		return err
	}
	connPaths, ok := connsV.Value().([]dbus.ObjectPath)
	if !ok {
		return fmt.Errorf("no connections found")
	}

	for _, cp := range connPaths {
		connObj := s.conn.Object(nmDest, cp)
		var settings map[string]map[string]dbus.Variant
		if err := connObj.Call("org.freedesktop.NetworkManager.Settings.Connection.GetSettings", 0).Store(&settings); err != nil {
			continue
		}
		if wireless, ok := settings["802-11-wireless"]; ok {
			if sv, ok := wireless["ssid"]; ok {
				if ssidBytes, ok := sv.Value().([]byte); ok && string(ssidBytes) == ssid {
					return connObj.Call("org.freedesktop.NetworkManager.Settings.Connection.Delete", 0).Err
				}
			}
		}
	}
	return fmt.Errorf("no saved connection for SSID %q", ssid)
}

// ConnectWithPassword creates a new connection with the given password and activates it.
// If password is empty, creates an open connection (no security).
func (s *Service) ConnectWithPassword(ssid, password string) error {
	apPath, err := s.findAccessPointPath(ssid)
	if err != nil {
		return err
	}
	devPath, err := s.findWifiDevicePath()
	if err != nil {
		return err
	}

	connection := map[string]map[string]dbus.Variant{
		"connection": {
			"id":   dbus.MakeVariant(ssid),
			"type": dbus.MakeVariant("802-11-wireless"),
		},
		"802-11-wireless": {
			"ssid": dbus.MakeVariant([]byte(ssid)),
			"mode": dbus.MakeVariant("infrastructure"),
		},
		"ipv4": {
			"method": dbus.MakeVariant("auto"),
		},
		"ipv6": {
			"method": dbus.MakeVariant("auto"),
		},
	}

	if password != "" {
		connection["802-11-wireless-security"] = map[string]dbus.Variant{
			"key-mgmt": dbus.MakeVariant("wpa-psk"),
			"psk":      dbus.MakeVariant(password),
		}
	}

	return s.conn.Object(nmDest, nmPath).Call(nmIface+".AddAndActivateConnection", 0, connection, devPath, apPath).Err
}

func (s *Service) findAccessPointPath(ssid string) (dbus.ObjectPath, error) {
	paths, err := s.wifiDevicePaths()
	if err != nil {
		return "/", err
	}
	for _, p := range paths {
		devObj := s.conn.Object(nmDest, p)
		apsV, err := devObj.GetProperty(nmDeviceWireless + ".AccessPoints")
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
			if string(ssidBytes) == ssid {
				return apPath, nil
			}
		}
	}
	return "/", fmt.Errorf("access point for SSID %q not found", ssid)
}

func (s *Service) findWifiDevicePath() (dbus.ObjectPath, error) {
	paths, err := s.wifiDevicePaths()
	if err != nil {
		return "/", err
	}
	if len(paths) == 0 {
		return "/", fmt.Errorf("no WiFi device found")
	}
	return paths[0], nil
}

// GetSavedSSIDs returns the SSIDs of all saved WiFi connections.
func (s *Service) GetSavedSSIDs() ([]string, error) {
	settingsObj := s.conn.Object(nmDest, "/org/freedesktop/NetworkManager/Settings")
	connsV, err := settingsObj.GetProperty("org.freedesktop.NetworkManager.Settings.Connections")
	if err != nil {
		return nil, err
	}
	connPaths, ok := connsV.Value().([]dbus.ObjectPath)
	if !ok {
		return nil, fmt.Errorf("no connections found")
	}

	var ssids []string
	for _, cp := range connPaths {
		connObj := s.conn.Object(nmDest, cp)
		var settings map[string]map[string]dbus.Variant
		if err := connObj.Call("org.freedesktop.NetworkManager.Settings.Connection.GetSettings", 0).Store(&settings); err != nil {
			continue
		}
		if wireless, ok := settings["802-11-wireless"]; ok {
			if sv, ok := wireless["ssid"]; ok {
				if ssidBytes, ok := sv.Value().([]byte); ok {
					ssids = append(ssids, string(ssidBytes))
				}
			}
		}
	}
	return ssids, nil
}
