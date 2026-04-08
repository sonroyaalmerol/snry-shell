package network

import (
	"context"
	"fmt"
	"log"
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
	// Emit initial state for subscribers.
	s.query()

	// Redundant: Manager now handles background updates and publishes to TopicNetwork.
	<-ctx.Done()
	return ctx.Err()
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

	primaryV, _ := nmObj.GetProperty(nmIface + ".PrimaryConnection")
	primaryPath, _ := primaryV.Value().(dbus.ObjectPath)

	ssid := ""
	connType := "none"
	var ipv4, ipv6, activeName string
	strength := 0

	// Always try to get SSID from any WiFi device regardless of primary connection
	if wifiPaths, err := s.wifiDevicePaths(); err == nil {
		for _, p := range wifiPaths {
			if s2, ok := s.getWifiSSID(p); ok {
				ssid = s2
				break
			}
		}
	}

	if primaryPath != "/" {
		primaryConnObj := s.conn.Object(nmDest, primaryPath)
		var connSettings map[string]map[string]dbus.Variant
		if err := primaryConnObj.Call("org.freedesktop.NetworkManager.Settings.Connection.GetSettings", 0).Store(&connSettings); err == nil {
			if c, ok := connSettings["connection"]; ok {
				activeName, _ = c["id"].Value().(string)
				rawType, _ := c["type"].Value().(string)
				if rawType == "802-11-wireless" {
					connType = "wifi"
				} else if rawType == "802-3-ethernet" {
					connType = "ethernet"
				} else {
					connType = rawType
				}
			}
		}

		// Find device for IP info
		devicesV, _ := nmObj.GetProperty(nmIface + ".Devices")
		if devices, ok := devicesV.Value().([]dbus.ObjectPath); ok {
			for _, devPath := range devices {
				devObj := s.conn.Object(nmDest, devPath)
				acV, _ := devObj.GetProperty("org.freedesktop.NetworkManager.Device.ActiveConnection")
				if acPath, ok := acV.Value().(dbus.ObjectPath); ok && acPath == primaryPath {
					if connType == "wifi" {
						if apV, err := devObj.GetProperty(nmDeviceWireless + ".ActiveAccessPoint"); err == nil {
							if apPath, ok := apV.Value().(dbus.ObjectPath); ok && apPath != "/" {
								apObj := s.conn.Object(nmDest, apPath)
								if strV, err := apObj.GetProperty(nmAccessPoint + ".Strength"); err == nil {
									if s, ok := strV.Value().(uint8); ok {
										strength = int(s)
									}
								}
							}
						}
					}

					// IP info
					if ip4V, err := devObj.GetProperty("org.freedesktop.NetworkManager.Device.Ip4Config"); err == nil {
						if ip4Path, ok := ip4V.Value().(dbus.ObjectPath); ok && ip4Path != "/" {
							ip4Obj := s.conn.Object(nmDest, ip4Path)
							if addrV, err := ip4Obj.GetProperty("org.freedesktop.NetworkManager.IP4Config.AddressData"); err == nil {
								if addrs, ok := addrV.Value().([]map[string]dbus.Variant); ok && len(addrs) > 0 {
									ipv4, _ = addrs[0]["address"].Value().(string)
								}
							}
						}
					}
					if ip6V, err := devObj.GetProperty("org.freedesktop.NetworkManager.Device.Ip6Config"); err == nil {
						if ip6Path, ok := ip6V.Value().(dbus.ObjectPath); ok && ip6Path != "/" {
							ip6Obj := s.conn.Object(nmDest, ip6Path)
							if addrV, err := ip6Obj.GetProperty("org.freedesktop.NetworkManager.IP6Config.AddressData"); err == nil {
								if addrs, ok := addrV.Value().([]map[string]dbus.Variant); ok && len(addrs) > 0 {
									ipv6, _ = addrs[0]["address"].Value().(string)
								}
							}
						}
					}
					break
				}
			}
		}
	}

	// Get wireless enabled state.
	wirelessV, err := nmObj.GetProperty(nmIface + ".WirelessEnabled")
	wirelessEnabled, _ := wirelessV.Value().(bool)

	return state.NetworkState{
		Type:                 connType,
		Connected:            connected,
		SSID:                 ssid,
		WirelessEnabled:      wirelessEnabled,
		Strength:             strength,
		IPv4:                 ipv4,
		IPv6:                 ipv6,
		ActiveConnectionName: activeName,
	}, nil
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
		dtV, err := devObj.GetProperty("org.freedesktop.NetworkManager.Device.DeviceType")
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
		err := devObj.Call(nmDeviceWireless+".RequestScan", 0, map[string]dbus.Variant{}).Err
		log.Printf("[WIFI] RequestScan on %s: %v", p, err)
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

	log.Printf("[WIFI] ScanWiFi: returning %d networks, current SSID=%q", len(networks), currentSSID)
	return networks, nil
}
func (s *Service) ConnectWiFi(ssid string) error {
	settingsObj := s.conn.Object(nmDest, "/org/freedesktop/NetworkManager/Settings")
	connsV, err := settingsObj.GetProperty("org.freedesktop.NetworkManager.Settings.Connections")
	if err != nil {
		return err
	}
	connPaths, ok := connsV.Value().([]dbus.ObjectPath)
	if !ok {
		return fmt.Errorf("no connections found")
	}

	// Find the connection for the requested SSID
	var targetConn dbus.ObjectPath
	for _, cp := range connPaths {
		connObj := s.conn.Object(nmDest, cp)
		var settings map[string]map[string]dbus.Variant
		if err := connObj.Call("org.freedesktop.NetworkManager.Settings.Connection.GetSettings", 0).Store(&settings); err != nil {
			continue
		}
		if wireless, ok := settings["802-11-wireless"]; ok {
			if sv, ok := wireless["ssid"]; ok {
				if ssidBytes, ok := sv.Value().([]byte); ok && string(ssidBytes) == ssid {
					targetConn = cp
					break
				}
			}
		}
	}

	if targetConn == "" {
		return fmt.Errorf("no existing connection for SSID %q", ssid)
	}

	// Find the wireless device to use
	nmObj := s.conn.Object(nmDest, nmPath)
	devicesV, err := nmObj.GetProperty(nmIface + ".Devices")
	if err != nil {
		return fmt.Errorf("failed to get devices: %w", err)
	}

	devicePaths, ok := devicesV.Value().([]dbus.ObjectPath)
	if !ok || len(devicePaths) == 0 {
		return fmt.Errorf("no network devices found")
	}

	// Find the first wireless device
	var wirelessDevice dbus.ObjectPath
	for _, devPath := range devicePaths {
		devObj := s.conn.Object(nmDest, devPath)
		dtV, err := devObj.GetProperty("org.freedesktop.NetworkManager.Device.DeviceType")
		if err != nil {
			continue
		}
		if dt, ok := dtV.Value().(uint32); ok && dt == 2 { // 2 = NM_DEVICE_TYPE_WIFI
			wirelessDevice = devPath
			break
		}
	}

	if wirelessDevice == "" {
		return fmt.Errorf("no wireless device found")
	}

	// Activate the connection on the specific device
	// This properly switches from one network to another
	call := nmObj.Call(nmIface+".ActivateConnection", 0, targetConn, wirelessDevice, dbus.ObjectPath("/"))
	if call.Err != nil {
		return fmt.Errorf("failed to activate connection: %w", call.Err)
	}

	return nil
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

// GetAllConnections returns all connection profiles from NetworkManager.
func (s *Service) GetAllConnections() ([]state.NMConnection, error) {
	settingsObj := s.conn.Object(nmDest, "/org/freedesktop/NetworkManager/Settings")
	connsV, err := settingsObj.GetProperty("org.freedesktop.NetworkManager.Settings.Connections")
	if err != nil {
		return nil, err
	}
	connPaths, ok := connsV.Value().([]dbus.ObjectPath)
	if !ok {
		return nil, fmt.Errorf("no connections found")
	}

	var connections []state.NMConnection
	for _, cp := range connPaths {
		connObj := s.conn.Object(nmDest, cp)
		var settings map[string]map[string]dbus.Variant
		if err := connObj.Call("org.freedesktop.NetworkManager.Settings.Connection.GetSettings", 0).Store(&settings); err != nil {
			continue
		}

		conn := state.NMConnection{
			Path: string(cp),
		}

		// Extract connection details
		if connSettings, ok := settings["connection"]; ok {
			if v, ok := connSettings["id"]; ok {
				conn.Name, _ = v.Value().(string)
			}
			if v, ok := connSettings["uuid"]; ok {
				conn.UUID, _ = v.Value().(string)
			}
			if v, ok := connSettings["type"]; ok {
				conn.Type, _ = v.Value().(string)
			}
			if v, ok := connSettings["autoconnect"]; ok {
				if ac, ok := v.Value().(bool); ok {
					conn.Autoconnect = ac
				}
			}
		}

		// Determine connection type name
		switch conn.Type {
		case "802-11-wireless":
			conn.TypeLabel = "Wi-Fi"
			if wireless, ok := settings["802-11-wireless"]; ok {
				if v, ok := wireless["ssid"]; ok {
					if ssidBytes, ok := v.Value().([]byte); ok {
						conn.SSID = string(ssidBytes)
					}
				}
			}
			// Check for security
			if sec, ok := settings["802-11-wireless-security"]; ok {
				if v, ok := sec["key-mgmt"]; ok {
					keyMgmt, _ := v.Value().(string)
					if keyMgmt == "wpa-psk" || keyMgmt == "wpa-eap" {
						conn.Secured = true
					}
				}
			}
		case "802-3-ethernet":
			conn.TypeLabel = "Ethernet"
			if eth, ok := settings["802-3-ethernet"]; ok {
				if v, ok := eth["mac-address"]; ok {
					if mac, ok := v.Value().([]byte); ok && len(mac) == 6 {
						conn.MAC = fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
					}
				}
			}
		case "vpn":
			conn.TypeLabel = "VPN"
			if vpn, ok := settings["vpn"]; ok {
				if v, ok := vpn["service-type"]; ok {
					conn.VPNType, _ = v.Value().(string)
				}
			}
		case "wireguard":
			conn.TypeLabel = "WireGuard"
		case "pppoe":
			conn.TypeLabel = "PPPoE"
		case "gsm", "cdma":
			conn.TypeLabel = "Mobile Broadband"
		default:
			conn.TypeLabel = conn.Type
		}

		// Get IP configuration
		if ipv4, ok := settings["ipv4"]; ok {
			if methodV, ok := ipv4["method"]; ok {
				conn.IPv4Method, _ = methodV.Value().(string)
			}
			if _, ok := ipv4["address-data"]; ok {
				conn.IPv4Configured = true
			}
		}
		if ipv6, ok := settings["ipv6"]; ok {
			if methodV, ok := ipv6["method"]; ok {
				conn.IPv6Method, _ = methodV.Value().(string)
			}
		}

		connections = append(connections, conn)
	}
	return connections, nil
}

// GetDevices returns all network devices managed by NetworkManager.
func (s *Service) GetDevices() ([]state.NMDevice, error) {
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

	var devices []state.NMDevice
	for _, p := range paths {
		devObj := s.conn.Object(nmDest, p)
		device := state.NMDevice{Path: string(p)}

		// Get device type
		if dtV, err := devObj.GetProperty("org.freedesktop.NetworkManager.Device.DeviceType"); err == nil {
			device.Type, _ = dtV.Value().(uint32)
		}

		// Get interface name
		if ifaceV, err := devObj.GetProperty("org.freedesktop.NetworkManager.Device.Interface"); err == nil {
			device.Interface, _ = ifaceV.Value().(string)
		}

		// Get state
		if stateV, err := devObj.GetProperty("org.freedesktop.NetworkManager.Device.State"); err == nil {
			device.State, _ = stateV.Value().(uint32)
		}

		// Get active connection
		if acV, err := devObj.GetProperty("org.freedesktop.NetworkManager.Device.ActiveConnection"); err == nil {
			if acPath, ok := acV.Value().(dbus.ObjectPath); ok && acPath != "/" {
				device.ActiveConnection = string(acPath)
			}
		}

		// Get IP config
		if ipV, err := devObj.GetProperty("org.freedesktop.NetworkManager.Device.Ip4Config"); err == nil {
			if ipPath, ok := ipV.Value().(dbus.ObjectPath); ok && ipPath != "/" {
				device.HasIP4 = true
			}
		}

		// Get MAC address for ethernet/wifi
		if device.Type == 1 || device.Type == 2 { // ethernet or wifi
			if hwV, err := devObj.GetProperty("org.freedesktop.NetworkManager.Device.HwAddress"); err == nil {
				device.HwAddress, _ = hwV.Value().(string)
			}
		}

		devices = append(devices, device)
	}
	return devices, nil
}

// ActivateConnection activates a connection profile on a specific device.
func (s *Service) ActivateConnection(connPath string, devicePath string) error {
	nmObj := s.conn.Object(nmDest, nmPath)
	if nmObj == nil {
		return fmt.Errorf("no D-Bus connection")
	}
	return nmObj.Call(nmIface+".ActivateConnection", 0,
		dbus.ObjectPath(connPath),
		dbus.ObjectPath(devicePath),
		dbus.ObjectPath("/")).Err
}

// DeactivateConnection deactivates an active connection.
func (s *Service) DeactivateConnection(connPath string) error {
	nmObj := s.conn.Object(nmDest, nmPath)
	if nmObj == nil {
		return fmt.Errorf("no D-Bus connection")
	}
	return nmObj.Call(nmIface+".DeactivateConnection", 0, dbus.ObjectPath(connPath)).Err
}

// DeleteConnection removes a saved connection profile.
func (s *Service) DeleteConnection(connPath string) error {
	connObj := s.conn.Object(nmDest, dbus.ObjectPath(connPath))
	return connObj.Call("org.freedesktop.NetworkManager.Settings.Connection.Delete", 0).Err
}

// UpdateConnection updates a connection's settings.
func (s *Service) UpdateConnection(connPath string, settings map[string]map[string]dbus.Variant) error {
	connObj := s.conn.Object(nmDest, dbus.ObjectPath(connPath))
	return connObj.Call("org.freedesktop.NetworkManager.Settings.Connection.Update", 0, settings).Err
}

// AddConnection adds a new connection profile.
func (s *Service) AddConnection(settings map[string]map[string]dbus.Variant) (string, error) {
	settingsObj := s.conn.Object(nmDest, "/org/freedesktop/NetworkManager/Settings")
	var newPath dbus.ObjectPath
	err := settingsObj.Call("org.freedesktop.NetworkManager.Settings.AddConnection", 0, settings).Store(&newPath)
	if err != nil {
		return "", err
	}
	return string(newPath), nil
}

// GetHostname returns the system hostname.
func (s *Service) GetHostname() (string, error) {
	nmObj := s.conn.Object(nmDest, nmPath)
	if nmObj == nil {
		return "", fmt.Errorf("no D-Bus connection")
	}
	hostnameV, err := nmObj.GetProperty(nmIface + ".Hostname")
	if err != nil {
		return "", err
	}
	hostname, _ := hostnameV.Value().(string)
	return hostname, nil
}

// SetHostname sets the system hostname.
func (s *Service) SetHostname(hostname string) error {
	nmObj := s.conn.Object(nmDest, nmPath)
	if nmObj == nil {
		return fmt.Errorf("no D-Bus connection")
	}
	return nmObj.SetProperty(nmIface+".Hostname", dbus.MakeVariant(hostname))
}

// SetAutoconnect enables/disables autoconnect for a connection.
func (s *Service) SetAutoconnect(connPath string, autoconnect bool) error {
	connObj := s.conn.Object(nmDest, dbus.ObjectPath(connPath))
	// Get current settings
	var settings map[string]map[string]dbus.Variant
	if err := connObj.Call("org.freedesktop.NetworkManager.Settings.Connection.GetSettings", 0).Store(&settings); err != nil {
		return err
	}
	// Update autoconnect
	if _, ok := settings["connection"]; !ok {
		settings["connection"] = make(map[string]dbus.Variant)
	}
	settings["connection"]["autoconnect"] = dbus.MakeVariant(autoconnect)
	// Update connection
	return connObj.Call("org.freedesktop.NetworkManager.Settings.Connection.Update", 0, settings).Err
}
