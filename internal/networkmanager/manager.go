// Package networkmanager provides a shared network management service
// that acts as a single source of truth for all network-related UI components.
package networkmanager

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
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
	nmSettingsPath    = "/org/freedesktop/NetworkManager/Settings"
	nmSettingsIface   = "org.freedesktop.NetworkManager.Settings"
	nmDeviceIface     = "org.freedesktop.NetworkManager.Device"
	nmDeviceWireless  = "org.freedesktop.NetworkManager.Device.Wireless"
	nmAccessPoint     = "org.freedesktop.NetworkManager.AccessPoint"
	nmConnectionIface = "org.freedesktop.NetworkManager.Settings.Connection"
	nmPropertiesIface = "org.freedesktop.DBus.Properties"
)

// Device type constants
const (
	DeviceTypeUnknown      = uint32(0)
	DeviceTypeEthernet     = uint32(1)
	DeviceTypeWiFi         = uint32(2)
	DeviceTypeBluetooth    = uint32(5)
	DeviceTypeOLPCMesh     = uint32(6)
	DeviceTypeWimax        = uint32(7)
	DeviceTypeModem        = uint32(8)
	DeviceTypeInfiniBand   = uint32(9)
	DeviceTypeBond         = uint32(10)
	DeviceTypeVLAN         = uint32(11)
	DeviceTypeADSL         = uint32(12)
	DeviceTypeBridge       = uint32(13)
	DeviceTypeGeneric      = uint32(14)
	DeviceTypeTeam         = uint32(15)
	DeviceTypeTUN          = uint32(16)
	DeviceTypeIPTunnel     = uint32(17)
	DeviceTypeMACVLAN      = uint32(18)
	DeviceTypeVXLAN        = uint32(19)
	DeviceTypeVETH         = uint32(20)
	DeviceTypeMACsec       = uint32(21)
	DeviceTypeDummy        = uint32(22)
	DeviceTypePPP          = uint32(23)
	DeviceTypeOVSInterface = uint32(24)
	DeviceTypeOVSPort      = uint32(25)
	DeviceTypeOVSBridge    = uint32(26)
	DeviceTypeOVSBond      = uint32(27)
	DeviceTypeWPAN         = uint32(28)
	DeviceType6LowPAN      = uint32(29)
	DeviceTypeWireGuard    = uint32(30)
	DeviceTypeWiFiP2P      = uint32(31)
	DeviceTypeVRF          = uint32(32)
)

// State constants
const (
	StateUnknown         = uint32(0)
	StateAsleep          = uint32(10)
	StateDisconnected    = uint32(20)
	StateDisconnecting   = uint32(30)
	StateConnecting      = uint32(40)
	StateConnectedLocal  = uint32(50)
	StateConnectedSite   = uint32(60)
	StateConnectedGlobal = uint32(70)
)

// Manager is the singleton network manager that provides a single source of truth
type Manager struct {
	conn dbusutil.DBusConn
	bus  *bus.Bus

	// Cached data
	mu               sync.RWMutex
	hostname         string
	state            uint32
	primaryConnection string
	devices          []state.NMDevice
	connections      []state.NMConnection
	wifiNetworks     []state.WiFiNetwork
	wirelessEnabled  bool

	// refreshMu serializes refreshAll so concurrent callers
	// (signal handler, periodic ticker, action methods) don't interleave.
	refreshMu sync.Mutex

	// Background refresh
	refreshCtx    context.Context
	refreshCancel context.CancelFunc
}

var (
	instance *Manager
	once     sync.Once
)

// GetInstance returns the singleton network manager instance
func GetInstance(dbusConn *dbus.Conn, b *bus.Bus) *Manager {
	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		instance = &Manager{
			conn:          dbusutil.NewRealConn(dbusConn),
			bus:           b,
			refreshCtx:    ctx,
			refreshCancel: cancel,
		}
		instance.start()
	})
	return instance
}

// start begins background monitoring
func (m *Manager) start() {
	// Initial fetch
	m.refreshAll()

	// Set up D-Bus signal monitoring
	if m.bus != nil {
		// Subscribe to NM state changes via bus
		go m.monitorDBusSignals()
	}

	// Periodic refresh as fallback
	go m.periodicRefresh()
}

// Stop stops the background monitoring
func (m *Manager) Stop() {
	m.refreshCancel()
}

// monitorDBusSignals listens for NM D-Bus signals
func (m *Manager) monitorDBusSignals() {
	ch := make(chan *dbus.Signal, 32)
	m.conn.Signal(ch)
	defer m.conn.RemoveSignal(ch)

	// Add match for NM signals
	busObj := m.conn.BusObject()
	busObj.Call("org.freedesktop.DBus.AddMatch", 0,
		"type='signal',sender='"+nmDest+"',interface='"+nmPropertiesIface+"',member='PropertiesChanged'")

	for {
		select {
		case <-m.refreshCtx.Done():
			return
		case sig, ok := <-ch:
			if !ok {
				return
			}
			// The D-Bus match rule already filtered by sender and
			// interface, so every signal in this channel is a valid
			// NM PropertiesChanged. No need to re-check sender/path
			// (sig.Sender is the unique bus name, not the well-known
			// name, so a direct comparison would incorrectly skip).
			//
			// Drain any queued signals before handling — NM often
			// sends many PropertiesChanged in rapid succession and
			// we only need to refresh once per batch.
			drain:
			for {
				select {
				case <-ch:
				default:
					break drain
				}
			}
			m.handleSignal(sig)
		}
	}
}

// handleSignal processes NM D-Bus signals
func (m *Manager) handleSignal(sig *dbus.Signal) {
	// Refresh data on any significant change
	m.refreshAll()

	// Publish update event
	if m.bus != nil {
		m.bus.Publish(bus.TopicNetworkManager, m.GetState())
	}
}

// periodicRefresh periodically refreshes data
func (m *Manager) periodicRefresh() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.refreshCtx.Done():
			return
		case <-ticker.C:
			m.refreshAll()
		}
	}
}

// refreshAll refreshes all cached data. It is serialized by refreshMu
// so concurrent callers (signal handler, periodic ticker) don't interleave.
func (m *Manager) refreshAll() {
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()

	m.refreshHostname()
	m.refreshState()
	m.refreshPrimaryConnection()
	m.refreshWirelessEnabled()
	// Devices first so connections can check active status.
	m.refreshDevices()
	m.refreshConnections()

	// Publish legacy state for backward compatibility
	if m.bus != nil {
		m.publishLegacyState()
	}
}

func (m *Manager) refreshWirelessEnabled() {
	nmObj := m.conn.Object(nmDest, nmPath)
	if nmObj == nil {
		return
	}

	wirelessV, err := nmObj.GetProperty(nmIface + ".WirelessEnabled")
	if err != nil {
		return
	}

	enabled, _ := wirelessV.Value().(bool)
	m.mu.Lock()
	m.wirelessEnabled = enabled
	m.mu.Unlock()
}

func (m *Manager) refreshPrimaryConnection() {
	nmObj := m.conn.Object(nmDest, nmPath)
	if nmObj == nil {
		return
	}

	primaryV, err := nmObj.GetProperty(nmIface + ".PrimaryConnection")
	if err != nil {
		return
	}

	path, _ := primaryV.Value().(dbus.ObjectPath)
	m.mu.Lock()
	m.primaryConnection = string(path)
	m.mu.Unlock()
}

// GetState returns current network state
func (m *Manager) GetState() state.NetworkManagerState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return state.NetworkManagerState{
		Hostname:        m.hostname,
		State:           m.state,
		Devices:         m.devices,
		Connections:     m.connections,
		WiFiNetworks:    m.wifiNetworks,
		WirelessEnabled: m.wirelessEnabled,
	}
}

// GetDevices returns all network devices
func (m *Manager) GetDevices() []state.NMDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.devices
}

// GetConnections returns all saved connections
func (m *Manager) GetConnections() []state.NMConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections
}

// GetWiFiNetworks returns cached WiFi networks
func (m *Manager) GetWiFiNetworks() []state.WiFiNetwork {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.wifiNetworks
}

// GetHostname returns system hostname
func (m *Manager) GetHostname() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hostname
}

// refreshHostname fetches hostname from NM
func (m *Manager) refreshHostname() {
	nmObj := m.conn.Object(nmDest, nmPath)
	if nmObj != nil {
		hostnameV, err := nmObj.GetProperty(nmIface + ".Hostname")
		if err == nil {
			m.mu.Lock()
			m.hostname, _ = hostnameV.Value().(string)
			m.mu.Unlock()
		}
	}

	m.mu.RLock()
	h := m.hostname
	m.mu.RUnlock()

	if h == "" {
		if h2, err := os.Hostname(); err == nil {
			m.mu.Lock()
			m.hostname = h2
			m.mu.Unlock()
		}
	}
}

// refreshState fetches NM state
func (m *Manager) refreshState() {
	nmObj := m.conn.Object(nmDest, nmPath)
	if nmObj == nil {
		return
	}

	stateV, err := nmObj.GetProperty(nmIface + ".State")
	if err != nil {
		return
	}

	m.mu.Lock()
	m.state, _ = stateV.Value().(uint32)
	m.mu.Unlock()
}

// refreshDevices fetches all devices
func (m *Manager) refreshDevices() {
	nmObj := m.conn.Object(nmDest, nmPath)
	if nmObj == nil {
		return
	}

	devicesV, err := nmObj.GetProperty(nmIface + ".Devices")
	if err != nil {
		return
	}

	paths, ok := devicesV.Value().([]dbus.ObjectPath)
	if !ok {
		return
	}

	var devices []state.NMDevice
	for _, p := range paths {
		dev := m.fetchDevice(p)
		if dev.Path != "" {
			devices = append(devices, dev)
		}
	}

	m.mu.Lock()
	m.devices = devices
	m.mu.Unlock()
}

// fetchDevice fetches a single device's info
func (m *Manager) fetchDevice(path dbus.ObjectPath) state.NMDevice {
	dev := state.NMDevice{Path: string(path)}
	devObj := m.conn.Object(nmDest, path)

	// Device type
	if dtV, err := devObj.GetProperty(nmDeviceIface + ".DeviceType"); err == nil {
		dev.Type, _ = dtV.Value().(uint32)
	}

	// Interface name
	if ifaceV, err := devObj.GetProperty(nmDeviceIface + ".Interface"); err == nil {
		dev.Interface, _ = ifaceV.Value().(string)
	}

	// State
	if stateV, err := devObj.GetProperty(nmDeviceIface + ".State"); err == nil {
		dev.State, _ = stateV.Value().(uint32)
	}

	// MAC address
	if macV, err := devObj.GetProperty(nmDeviceIface + ".HwAddress"); err == nil {
		dev.HwAddress, _ = macV.Value().(string)
	}

	// IP4 config
	if ip4V, err := devObj.GetProperty(nmDeviceIface + ".Ip4Config"); err == nil {
		if ip4Path, ok := ip4V.Value().(dbus.ObjectPath); ok && ip4Path != "/" {
			dev.HasIP4 = true
		}
	}

	// IP6 config
	if ip6V, err := devObj.GetProperty(nmDeviceIface + ".Ip6Config"); err == nil {
		if ip6Path, ok := ip6V.Value().(dbus.ObjectPath); ok && ip6Path != "/" {
			dev.HasIP6 = true
		}
	}

	// Active connection
	if acV, err := devObj.GetProperty(nmDeviceIface + ".ActiveConnection"); err == nil {
		if acPath, ok := acV.Value().(dbus.ObjectPath); ok && acPath != "/" {
			dev.ActiveConnection = string(acPath)
			// Fetch connection name
			if conn := m.getConnectionByPath(string(acPath)); conn != nil {
				dev.ActiveConnectionName = conn.Name
			}
		}
	}

	// For WiFi devices, get additional info
	if dev.Type == DeviceTypeWiFi {
		if apV, err := devObj.GetProperty(nmDeviceWireless + ".ActiveAccessPoint"); err == nil {
			if apPath, ok := apV.Value().(dbus.ObjectPath); ok && apPath != "/" {
				apObj := m.conn.Object(nmDest, apPath)
				if ssidV, err := apObj.GetProperty(nmAccessPoint + ".Ssid"); err == nil {
					if ssidBytes, ok := ssidV.Value().([]byte); ok {
						dev.ActiveSSID = string(ssidBytes)
					}
				}
				if strengthV, err := apObj.GetProperty(nmAccessPoint + ".Strength"); err == nil {
					dev.SignalStrength, _ = strengthV.Value().(uint8)
				}
			}
		}
	}

	return dev
}

// refreshConnections fetches all saved connections
func (m *Manager) refreshConnections() {
	settingsObj := m.conn.Object(nmDest, nmSettingsPath)
	if settingsObj == nil {
		return
	}

	connsV, err := settingsObj.GetProperty(nmSettingsIface + ".Connections")
	if err != nil {
		return
	}

	paths, ok := connsV.Value().([]dbus.ObjectPath)
	if !ok {
		return
	}

	var connections []state.NMConnection
	for _, p := range paths {
		conn := m.fetchConnection(p)
		if conn.Path != "" {
			connections = append(connections, conn)
		}
	}

	m.mu.Lock()
	m.connections = connections
	m.mu.Unlock()
}

// fetchConnection fetches a single connection's settings
func (m *Manager) fetchConnection(path dbus.ObjectPath) state.NMConnection {
	conn := state.NMConnection{Path: string(path), Autoconnect: true}
	connObj := m.conn.Object(nmDest, path)

	var settings map[string]map[string]dbus.Variant
	if err := connObj.Call(nmConnectionIface+".GetSettings", 0).Store(&settings); err != nil {
		return conn
	}

	// Parse connection settings
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
			conn.Autoconnect, _ = v.Value().(bool)
		}
		if v, ok := connSettings["interface-name"]; ok {
			conn.InterfaceName, _ = v.Value().(string)
		}
		if v, ok := connSettings["timestamp"]; ok {
			if ts, ok := v.Value().(uint64); ok {
				conn.LastUsed = time.Unix(int64(ts), 0)
			}
		}
	}

	// Determine type label and extract type-specific info
	switch conn.Type {
	case "802-11-wireless":
		conn.TypeLabel = "Wi-Fi"
		if wireless, ok := settings["802-11-wireless"]; ok {
			if v, ok := wireless["ssid"]; ok {
				if ssidBytes, ok := v.Value().([]byte); ok {
					conn.SSID = string(ssidBytes)
				}
			}
			if v, ok := wireless["mode"]; ok {
				conn.WiFiMode, _ = v.Value().(string)
			}
		}
		// Check security
		if sec, ok := settings["802-11-wireless-security"]; ok {
			conn.Secured = true
			if v, ok := sec["key-mgmt"]; ok {
				conn.SecurityType, _ = v.Value().(string)
			}
		}

	case "802-3-ethernet":
		conn.TypeLabel = "Ethernet"
		if eth, ok := settings["802-3-ethernet"]; ok {
			if v, ok := eth["mac-address"]; ok {
				if mac, ok := v.Value().([]byte); ok && len(mac) == 6 {
					conn.MAC = fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
						mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
				}
			}
		}

	case "vpn":
		conn.TypeLabel = "VPN"
		if vpn, ok := settings["vpn"]; ok {
			if v, ok := vpn["service-type"]; ok {
				conn.VPNType, _ = v.Value().(string)
				// Extract VPN type name
				switch conn.VPNType {
				case "org.freedesktop.NetworkManager.openvpn":
					conn.VPNTypeLabel = "OpenVPN"
				case "org.freedesktop.NetworkManager.wireguard":
					conn.VPNTypeLabel = "WireGuard"
				case "org.freedesktop.NetworkManager.strongswan":
					conn.VPNTypeLabel = "Strongswan"
				default:
					conn.VPNTypeLabel = "VPN"
				}
			}
		}

	case "wireguard":
		conn.TypeLabel = "WireGuard"

	case "pppoe":
		conn.TypeLabel = "PPPoE"

	case "gsm", "cdma":
		conn.TypeLabel = "Mobile Broadband"
		if gsm, ok := settings["gsm"]; ok {
			if v, ok := gsm["apn"]; ok {
				conn.APN, _ = v.Value().(string)
			}
		}

	default:
		conn.TypeLabel = conn.Type
	}

	// IP configuration
	if ipv4, ok := settings["ipv4"]; ok {
		if v, ok := ipv4["method"]; ok {
			conn.IPv4Method, _ = v.Value().(string)
		}
		// Check if addresses are configured
		if v, ok := ipv4["address-data"]; ok {
			if addrs, ok := v.Value().([]map[string]dbus.Variant); ok && len(addrs) > 0 {
				conn.IPv4Configured = true
				conn.IPv4Address, _ = addrs[0]["address"].Value().(string)
			}
		}
		if v, ok := ipv4["gateway"]; ok {
			conn.IPv4Gateway, _ = v.Value().(string)
		}
		if v, ok := ipv4["dns"]; ok {
			if dns, ok := v.Value().([]uint32); ok && len(dns) > 0 {
				conn.IPv4DNSConfigured = true
				for _, d := range dns {
					// NetworkManager stores IPv4 as uint32
					conn.IPv4DNS = append(conn.IPv4DNS, fmt.Sprintf("%d.%d.%d.%d",
						byte(d), byte(d>>8), byte(d>>16), byte(d>>24)))
				}
			}
		}
	}

	if ipv6, ok := settings["ipv6"]; ok {
		if v, ok := ipv6["method"]; ok {
			conn.IPv6Method, _ = v.Value().(string)
		}
		if v, ok := ipv6["address-data"]; ok {
			if addrs, ok := v.Value().([]map[string]dbus.Variant); ok && len(addrs) > 0 {
				conn.IPv6Configured = true
				conn.IPv6Address, _ = addrs[0]["address"].Value().(string)
			}
		}
		if v, ok := ipv6["gateway"]; ok {
			conn.IPv6Gateway, _ = v.Value().(string)
		}
	}

	// Check if connection is active
	m.mu.RLock()
	primary := m.primaryConnection
	devices := m.devices
	m.mu.RUnlock()

	for _, dev := range devices {
		if dev.ActiveConnection == conn.Path {
			conn.Active = true
			if conn.Path == primary {
				conn.IsPrimary = true
			}
			break
		}
	}

	return conn
}

// getConnectionByPath finds a connection by its D-Bus path
func (m *Manager) getConnectionByPath(path string) *state.NMConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.connections {
		if m.connections[i].Path == path {
			return &m.connections[i]
		}
	}
	return nil
}

// SetHostname sets the system hostname via NM's Settings.SaveHostname.
func (m *Manager) SetHostname(hostname string) error {
	settingsObj := m.conn.Object(nmDest, nmSettingsPath)
	if settingsObj == nil {
		return fmt.Errorf("no D-Bus connection")
	}

	if err := settingsObj.Call(nmSettingsIface+".SaveHostname", 0, hostname).Err; err != nil {
		return err
	}

	// Refresh cache
	m.refreshHostname()

	// Publish update
	if m.bus != nil {
		m.bus.Publish(bus.TopicNetworkManager, m.GetState())
	}

	return nil
}

// SetAutoconnect enables/disables autoconnect for a connection
func (m *Manager) SetAutoconnect(connPath string, autoconnect bool) error {
	connObj := m.conn.Object(nmDest, dbus.ObjectPath(connPath))

	// Get current settings
	var settings map[string]map[string]dbus.Variant
	if err := connObj.Call(nmConnectionIface+".GetSettings", 0).Store(&settings); err != nil {
		return err
	}

	// Update autoconnect
	if _, ok := settings["connection"]; !ok {
		settings["connection"] = make(map[string]dbus.Variant)
	}
	settings["connection"]["autoconnect"] = dbus.MakeVariant(autoconnect)

	// Update connection
	if err := connObj.Call(nmConnectionIface+".Update", 0, settings).Err; err != nil {
		return err
	}

	// Refresh cache
	m.refreshConnections()

	// Publish update
	if m.bus != nil {
		m.bus.Publish(bus.TopicNetworkManager, m.GetState())
		m.publishLegacyState()
	}

	return nil
}

// publishLegacyState publishes to the old TopicNetwork for backward compatibility
func (m *Manager) publishLegacyState() {
	m.mu.RLock()
	state_val := m.state
	devices := m.devices
	wirelessEnabled := m.wirelessEnabled
	m.mu.RUnlock()

	nmObj := m.conn.Object(nmDest, nmPath)
	primaryV, _ := nmObj.GetProperty(nmIface + ".PrimaryConnection")
	primaryPath, _ := primaryV.Value().(dbus.ObjectPath)

	var primaryConn *state.NMConnection
	if primaryPath != "/" {
		primaryConn = m.getConnectionByPath(string(primaryPath))
	}

	// Legacy defaults
	ssid := ""
	strength := 0
	connected := state_val >= StateConnectedLocal
	connType := "none"
	var ipv4, ipv6, activeName string

	if primaryConn != nil {
		activeName = primaryConn.Name
		if primaryConn.Type == "802-11-wireless" {
			connType = "wifi"
			ssid = primaryConn.SSID
		} else if primaryConn.Type == "802-3-ethernet" {
			connType = "ethernet"
		} else {
			connType = primaryConn.TypeLabel
		}

		// Find the device associated with this primary connection to get IP info and signal strength
		for _, dev := range devices {
			if dev.ActiveConnection == string(primaryPath) {
				if connType == "wifi" {
					strength = int(dev.SignalStrength)
				}
				// Fetch IP info from the device object
				devObj := m.conn.Object(nmDest, dbus.ObjectPath(dev.Path))
				if ip4V, err := devObj.GetProperty(nmDeviceIface + ".Ip4Config"); err == nil {
					if ip4Path, ok := ip4V.Value().(dbus.ObjectPath); ok && ip4Path != "/" {
						ip4Obj := m.conn.Object(nmDest, ip4Path)
						if addrV, err := ip4Obj.GetProperty("org.freedesktop.NetworkManager.IP4Config.AddressData"); err == nil {
							if addrs, ok := addrV.Value().([]map[string]dbus.Variant); ok && len(addrs) > 0 {
								ipv4, _ = addrs[0]["address"].Value().(string)
							}
						}
					}
				}
				if ip6V, err := devObj.GetProperty(nmDeviceIface + ".Ip6Config"); err == nil {
					if ip6Path, ok := ip6V.Value().(dbus.ObjectPath); ok && ip6Path != "/" {
						ip6Obj := m.conn.Object(nmDest, ip6Path)
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

	legacyState := state.NetworkState{
		Type:                 connType,
		SSID:                 ssid,
		Connected:            connected,
		Strength:             strength,
		WirelessEnabled:      wirelessEnabled,
		IPv4:                 ipv4,
		IPv6:                 ipv6,
		ActiveConnectionName: activeName,
	}

	m.bus.Publish(bus.TopicNetwork, legacyState)
}
func (m *Manager) DeleteConnection(connPath string) error {
	connObj := m.conn.Object(nmDest, dbus.ObjectPath(connPath))

	if err := connObj.Call(nmConnectionIface+".Delete", 0).Err; err != nil {
		return err
	}

	// Refresh cache
	m.refreshConnections()

	// Publish update
	if m.bus != nil {
		m.bus.Publish(bus.TopicNetworkManager, m.GetState())
	}

	return nil
}

// ActivateConnection activates a connection on a device
func (m *Manager) ActivateConnection(connPath string, devicePath string) error {
	nmObj := m.conn.Object(nmDest, nmPath)
	if nmObj == nil {
		return fmt.Errorf("no D-Bus connection")
	}

	var specificPath dbus.ObjectPath = "/"

	return nmObj.Call(nmIface+".ActivateConnection", 0,
		dbus.ObjectPath(connPath),
		dbus.ObjectPath(devicePath),
		specificPath).Err
}

// DeactivateConnection deactivates an active connection
func (m *Manager) DeactivateConnection(activeConnPath string) error {
	nmObj := m.conn.Object(nmDest, nmPath)
	if nmObj == nil {
		return fmt.Errorf("no D-Bus connection")
	}

	return nmObj.Call(nmIface+".DeactivateConnection", 0, dbus.ObjectPath(activeConnPath)).Err
}

// AddConnection creates a new connection
func (m *Manager) AddConnection(settings map[string]map[string]dbus.Variant) (string, error) {
	settingsObj := m.conn.Object(nmDest, nmSettingsPath)

	var newPath dbus.ObjectPath
	err := settingsObj.Call(nmSettingsIface+".AddConnection", 0, settings).Store(&newPath)
	if err != nil {
		return "", err
	}

	// Refresh cache
	m.refreshConnections()

	// Publish update
	if m.bus != nil {
		m.bus.Publish(bus.TopicNetworkManager, m.GetState())
	}

	return string(newPath), nil
}

// UpdateConnection updates an existing connection
func (m *Manager) UpdateConnection(connPath string, settings map[string]map[string]dbus.Variant) error {
	connObj := m.conn.Object(nmDest, dbus.ObjectPath(connPath))

	if err := connObj.Call(nmConnectionIface+".Update", 0, settings).Err; err != nil {
		return err
	}

	// Refresh cache
	m.refreshConnections()

	// Publish update
	if m.bus != nil {
		m.bus.Publish(bus.TopicNetworkManager, m.GetState())
	}

	return nil
}

// ScanWiFi triggers a WiFi scan and returns available networks
func (m *Manager) ScanWiFi() ([]state.WiFiNetwork, error) {
	// Find WiFi devices
	var wifiDevices []string
	m.mu.RLock()
	for _, dev := range m.devices {
		if dev.Type == DeviceTypeWiFi {
			wifiDevices = append(wifiDevices, dev.Path)
		}
	}
	m.mu.RUnlock()

	if len(wifiDevices) == 0 {
		return nil, fmt.Errorf("no WiFi devices found")
	}

	// Request scan on all WiFi devices
	for _, path := range wifiDevices {
		devObj := m.conn.Object(nmDest, dbus.ObjectPath(path))
		err := devObj.Call(nmDeviceWireless+".RequestScan", 0, map[string]dbus.Variant{}).Err
		if err != nil {
			log.Printf("[NetworkManager] WiFi scan request failed for %s: %v", path, err)
		}
	}

	// Wait for scan to complete
	time.Sleep(3 * time.Second)

	// Fetch scan results
	networks := m.fetchWiFiNetworks()

	m.mu.Lock()
	m.wifiNetworks = networks
	m.mu.Unlock()

	// Publish update
	if m.bus != nil {
		m.bus.Publish(bus.TopicNetworkManager, m.GetState())
	}

	return networks, nil
}

// fetchWiFiNetworks fetches available WiFi networks from scan results
func (m *Manager) fetchWiFiNetworks() []state.WiFiNetwork {
	var networks []state.WiFiNetwork
	seen := make(map[string]bool)

	// Get current connected SSID
	currentSSID := ""
	m.mu.RLock()
	for _, dev := range m.devices {
		if dev.Type == DeviceTypeWiFi {
			currentSSID = dev.ActiveSSID
			break
		}
	}
	m.mu.RUnlock()

	// Get saved SSIDs
	savedSSIDs := make(map[string]bool)
	m.mu.RLock()
	for _, conn := range m.connections {
		if conn.Type == "802-11-wireless" && conn.SSID != "" {
			savedSSIDs[conn.SSID] = true
		}
	}
	m.mu.RUnlock()

	// Scan all WiFi devices
	m.mu.RLock()
	devices := m.devices
	m.mu.RUnlock()

	for _, dev := range devices {
		if dev.Type != DeviceTypeWiFi {
			continue
		}

		devObj := m.conn.Object(nmDest, dbus.ObjectPath(dev.Path))

		apsV, err := devObj.GetProperty(nmDeviceWireless + ".AccessPoints")
		if err != nil {
			continue
		}

		apPaths, ok := apsV.Value().([]dbus.ObjectPath)
		if !ok {
			continue
		}

		for _, apPath := range apPaths {
			apObj := m.conn.Object(nmDest, apPath)

			// Get SSID
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

			// Get signal strength
			strength := 0
			if strV, err := apObj.GetProperty(nmAccessPoint + ".Strength"); err == nil {
				if v, ok := strV.Value().(uint8); ok {
					strength = int(v)
				}
			}

			// Get security
			security := ""
			if flagsV, err := apObj.GetProperty(nmAccessPoint + ".WpaFlags"); err == nil {
				if flags, ok := flagsV.Value().(uint32); ok && flags > 0 {
					security = "WPA"
				}
			}
			if rsnV, err := apObj.GetProperty(nmAccessPoint + ".RsnFlags"); err == nil {
				if flags, ok := rsnV.Value().(uint32); ok && flags > 0 {
					if security != "" {
						security = "WPA/WPA2"
					} else {
						security = "WPA2"
					}
				}
			}

			networks = append(networks, state.WiFiNetwork{
				SSID:      ssid,
				Signal:    strength,
				Security:  security,
				Connected: ssid == currentSSID,
				Saved:     savedSSIDs[ssid],
			})
		}
	}

	return networks
}

// SetWirelessEnabled enables/disables WiFi and immediately publishes
// both TopicNetworkManager and TopicNetwork so the bar icon updates
// without waiting for a D-Bus PropertiesChanged round-trip.
func (m *Manager) SetWirelessEnabled(enabled bool) error {
	nmObj := m.conn.Object(nmDest, nmPath)
	if nmObj == nil {
		return fmt.Errorf("no D-Bus connection")
	}

	err := nmObj.SetProperty(nmIface+".WirelessEnabled", dbus.MakeVariant(enabled))
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.wirelessEnabled = enabled
	m.mu.Unlock()

	if m.bus != nil {
		m.publishLegacyState()
		m.bus.Publish(bus.TopicNetworkManager, m.GetState())
	}

	return nil
}

// ConnectWiFi connects to a WiFi network using saved credentials
func (m *Manager) ConnectWiFi(ssid string) error {
	// Find connection for this SSID
	m.mu.RLock()
	var connPath string
	for _, conn := range m.connections {
		if conn.SSID == ssid {
			connPath = conn.Path
			break
		}
	}
	m.mu.RUnlock()

	if connPath == "" {
		return fmt.Errorf("no saved connection for SSID %q", ssid)
	}

	// Find WiFi device
	m.mu.RLock()
	var devicePath string
	for _, dev := range m.devices {
		if dev.Type == DeviceTypeWiFi {
			devicePath = dev.Path
			break
		}
	}
	m.mu.RUnlock()

	if devicePath == "" {
		return fmt.Errorf("no WiFi device found")
	}

	return m.ActivateConnection(connPath, devicePath)
}

// ConnectWiFiWithPassword creates and connects to a new WiFi network
func (m *Manager) ConnectWiFiWithPassword(ssid, password string) error {
	// Find WiFi device
	m.mu.RLock()
	var devicePath string
	for _, dev := range m.devices {
		if dev.Type == DeviceTypeWiFi {
			devicePath = dev.Path
			break
		}
	}
	m.mu.RUnlock()

	if devicePath == "" {
		return fmt.Errorf("no WiFi device found")
	}

	// Find the access point path by scanning the device's AP list.
	var apPath dbus.ObjectPath = "/"
	devObj := m.conn.Object(nmDest, dbus.ObjectPath(devicePath))
	if apsV, err := devObj.GetProperty(nmDeviceWireless + ".AccessPoints"); err == nil {
		if apPaths, ok := apsV.Value().([]dbus.ObjectPath); ok {
			for _, p := range apPaths {
				apObj := m.conn.Object(nmDest, p)
				if ssidV, err := apObj.GetProperty(nmAccessPoint + ".Ssid"); err == nil {
					if b, ok := ssidV.Value().([]byte); ok && string(b) == ssid {
						apPath = p
						break
					}
				}
			}
		}
	}

	// Build connection settings
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

	// Add security if password provided
	if password != "" {
		connection["802-11-wireless-security"] = map[string]dbus.Variant{
			"key-mgmt": dbus.MakeVariant("wpa-psk"),
			"psk":      dbus.MakeVariant(password),
		}
	}

	// Add and activate
	nmObj := m.conn.Object(nmDest, nmPath)
	return nmObj.Call(nmIface+".AddAndActivateConnection", 0,
		connection,
		dbus.ObjectPath(devicePath),
		apPath).Err
}
