package network

import (
	"context"
	"fmt"
	"log"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/dbusutil"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const (
	nmDest           = "org.freedesktop.NetworkManager"
	nmPath           = "/org/freedesktop/NetworkManager"
	nmIface          = "org.freedesktop.NetworkManager"
	nmDeviceWireless = "org.freedesktop.NetworkManager.Device.Wireless"
	nmAccessPoint    = "org.freedesktop.NetworkManager.AccessPoint"
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

	// Emit initial state for subscribers.
	ns := s.fetchFullState()
	s.bus.Publish(bus.TopicNetwork, ns)

	// Monitor D-Bus signals for live updates.
	go s.monitorSignals(ctx)

	<-ctx.Done()
	return ctx.Err()
}

func (s *Service) monitorSignals(ctx context.Context) {
	if s.conn == nil {
		return
	}

	ch := make(chan *dbus.Signal, 32)
	s.conn.Signal(ch)
	defer s.conn.RemoveSignal(ch)

	busObj := s.conn.BusObject()
	busObj.Call("org.freedesktop.DBus.AddMatch", 0,
		"type='signal',sender='"+nmDest+"',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged'")

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			// Drain queued signals before handling.
			drain:
			for {
				select {
				case <-ch:
				default:
					break drain
				}
			}
			ns := s.fetchFullState()
			s.bus.Publish(bus.TopicNetwork, ns)
		}
	}
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
		// Resolve the settings path from the active connection.
		acObj := s.conn.Object(nmDest, primaryPath)
		settingsPathV, _ := acObj.GetProperty("org.freedesktop.NetworkManager.Connection.Active.Connection")
		if settingsPath, ok := settingsPathV.Value().(dbus.ObjectPath); ok && settingsPath != "/" {
			settingsObj := s.conn.Object(nmDest, settingsPath)
			var connSettings map[string]map[string]dbus.Variant
			if err := settingsObj.Call("org.freedesktop.NetworkManager.Settings.Connection.GetSettings", 0).Store(&connSettings); err == nil {
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
									if v, ok := strV.Value().(uint8); ok {
										strength = int(v)
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
	var wirelessEnabled bool
	if wirelessV, err := nmObj.GetProperty(nmIface + ".WirelessEnabled"); err == nil {
		wirelessEnabled, _ = wirelessV.Value().(bool)
	}

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

// fetchFullState returns NetworkState enriched with WiFi scan results.
// It stamps Connected=true on the WiFiNetwork matching the current SSID.
func (s *Service) fetchFullState() state.NetworkState {
	ns, err := s.fetchState()
	if err != nil {
		return ns
	}

	networks := s.scanAccessPoints()
	for i := range networks {
		networks[i].Connected = (networks[i].SSID == ns.SSID)
	}
	ns.WiFiNetworks = networks
	return ns
}

// scanAccessPoints returns visible WiFi access points without publishing.
func (s *Service) scanAccessPoints() []state.WiFiNetwork {
	if s.conn == nil {
		return nil
	}

	savedSet := make(map[string]bool)
	if saved, err := s.GetSavedSSIDs(); err == nil {
		for _, ssid := range saved {
			savedSet[ssid] = true
		}
	}

	paths, err := s.wifiDevicePaths()
	if err != nil || len(paths) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var networks []state.WiFiNetwork

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
			apSSID := string(ssidBytes)
			if apSSID == "" || seen[apSSID] {
				continue
			}
			seen[apSSID] = true

			sig := 0
			if strV, err := apObj.GetProperty(nmAccessPoint + ".Strength"); err == nil {
				if v, ok := strV.Value().(uint8); ok {
					sig = int(v)
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
				SSID:     apSSID,
				Signal:   sig,
				Security: security,
				Saved:    savedSet[apSSID],
			})
		}
	}

	return networks
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

// ScanWiFi triggers a WiFi scan via NetworkManager.
// Results arrive asynchronously via D-Bus signals which trigger fetchFullState.
func (s *Service) ScanWiFi(ctx context.Context) ([]state.WiFiNetwork, error) {
	if s.conn == nil {
		return nil, fmt.Errorf("no D-Bus connection")
	}

	paths, err := s.wifiDevicePaths()
	if err != nil {
		return nil, err
	}

	for _, p := range paths {
		devObj := s.conn.Object(nmDest, p)
		err := devObj.Call(nmDeviceWireless+".RequestScan", 0, map[string]dbus.Variant{}).Err
		if err != nil {
			log.Printf("[network] RequestScan on %s: %v", p, err)
		}
	}

	// Publish current AP list immediately (no wait).
	ns := s.fetchFullState()
	s.bus.Publish(bus.TopicNetwork, ns)

	return ns.WiFiNetworks, nil
}

func (s *Service) ConnectWiFi(ssid string) error {
	if s.conn == nil {
		return fmt.Errorf("no D-Bus connection")
	}

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

	// Find the wireless device to use (reuse wifiDevicePaths).
	wifiPaths, err := s.wifiDevicePaths()
	if err != nil {
		return err
	}
	if len(wifiPaths) == 0 {
		return fmt.Errorf("no wireless device found")
	}

	nmObj := s.conn.Object(nmDest, nmPath)
	call := nmObj.Call(nmIface+".ActivateConnection", 0, targetConn, wifiPaths[0], dbus.ObjectPath("/"))
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
	if err := nmObj.Call(nmIface+".DeactivateConnection", 0, primaryPath).Err; err != nil {
		return err
	}
	return nil
}

// ForgetWiFi removes the saved connection profile for the given SSID.
func (s *Service) ForgetWiFi(ssid string) error {
	if s.conn == nil {
		return fmt.Errorf("no D-Bus connection")
	}
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
	if s.conn == nil {
		return nil, fmt.Errorf("no D-Bus connection")
	}
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
	if s.conn == nil {
		return nil, fmt.Errorf("no D-Bus connection")
	}
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
	if s.conn == nil {
		return fmt.Errorf("no D-Bus connection")
	}
	connObj := s.conn.Object(nmDest, dbus.ObjectPath(connPath))
	return connObj.Call("org.freedesktop.NetworkManager.Settings.Connection.Delete", 0).Err
}

// UpdateConnection updates a connection's settings.
func (s *Service) UpdateConnection(connPath string, settings map[string]map[string]dbus.Variant) error {
	if s.conn == nil {
		return fmt.Errorf("no D-Bus connection")
	}
	connObj := s.conn.Object(nmDest, dbus.ObjectPath(connPath))
	return connObj.Call("org.freedesktop.NetworkManager.Settings.Connection.Update", 0, settings).Err
}

// AddConnection adds a new connection profile.
func (s *Service) AddConnection(settings map[string]map[string]dbus.Variant) (string, error) {
	if s.conn == nil {
		return "", fmt.Errorf("no D-Bus connection")
	}
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
	if s.conn == nil {
		return fmt.Errorf("no D-Bus connection")
	}
	settingsObj := s.conn.Object(nmDest, "/org/freedesktop/NetworkManager/Settings")
	return settingsObj.Call("org.freedesktop.NetworkManager.Settings.SaveHostname", 0, hostname).Err
}

// SetAutoconnect enables/disables autoconnect for a connection.
func (s *Service) SetAutoconnect(connPath string, autoconnect bool) error {
	if s.conn == nil {
		return fmt.Errorf("no D-Bus connection")
	}
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
