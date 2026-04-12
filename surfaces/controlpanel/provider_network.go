package controlpanel

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/controlsocket"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/networkmanager"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// nmConfigProvider implements ConfigProvider for NetworkManager settings
type nmConfigProvider struct {
	conn    *dbus.Conn
	manager *networkmanager.Manager
	// Keyed lists for diff-based updates
	devicesKL *gtkutil.KeyedList[state.NMDevice]
	wifiKL    *gtkutil.KeyedList[state.WiFiNetwork]
	connKL    *gtkutil.KeyedList[state.NMConnection]
	// Widgets that need updates
	devicesList     *gtk.Box
	wifiList        *gtk.Box
	connectionsList *gtk.Box
	hostnameEntry   *gtk.Entry
}

func newNMProviderWithConnection() ConfigProvider {
	// Connect to D-Bus
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Printf("[CONTROLPANEL] cannot connect to system bus: %v", err)
		return nil
	}

	// Get the singleton manager (nil bus since control panel has no shell bus)
	manager := networkmanager.GetInstance(conn, nil)

	return &nmConfigProvider{
		conn:    conn,
		manager: manager,
	}
}

func (n *nmConfigProvider) Name() string {
	return "Network"
}

func (n *nmConfigProvider) Icon() string {
	return "settings_ethernet"
}

func (n *nmConfigProvider) Load() error {
	return nil
}

func (n *nmConfigProvider) Save() error {
	return nil
}

func (n *nmConfigProvider) Close() {
	if n.conn != nil {
		n.conn.Close()
	}
}

func (n *nmConfigProvider) notifyShellReload() {
	conn, err := net.Dial("unix", controlsocket.DefaultPath)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.Write([]byte("reload-settings"))
}

func (n *nmConfigProvider) BuildWidget() gtk.Widgetter {
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("popup-scroll")
	scroll.SetVExpand(true)
	scroll.SetHExpand(true)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("settings-stack")
	box.SetVExpand(true)
	box.SetHExpand(true)
	box.SetMarginTop(12)
	box.SetMarginBottom(24)
	box.SetMarginStart(24)
	box.SetMarginEnd(24)

	// Title
	title := gtk.NewLabel("Network Settings")
	title.AddCSSClass("settings-title")
	title.SetHAlign(gtk.AlignStart)
	box.Append(title)

	// Build sections
	box.Append(n.buildHostnameSection())
	box.Append(n.buildDevicesSection())
	box.Append(n.buildWiFiSection())
	box.Append(n.buildConnectionsSection())

	scroll.SetChild(box)

	// Monitor NM D-Bus signals for real-time updates
	go n.monitorSignals()

	return scroll
}

func (n *nmConfigProvider) buildHostnameSection() gtk.Widgetter {
	section := gtk.NewBox(gtk.OrientationVertical, 12)
	section.AddCSSClass("settings-section")
	section.SetMarginTop(24)

	subtitle := gtk.NewLabel("System Hostname")
	subtitle.AddCSSClass("settings-subtitle")
	subtitle.SetHAlign(gtk.AlignStart)
	section.Append(subtitle)

	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("system-controls")

	row := gtk.NewBox(gtk.OrientationHorizontal, 16)
	row.AddCSSClass("m3-switch-row")

	hostnameField := gtkutil.NewM3OutlinedTextField()
	hostnameField.SetText(n.manager.GetHostname())
	n.hostnameEntry = hostnameField.Entry()
	hostnameField.SetHExpand(true)

	updateBtn := gtkutil.M3IconButton("check", "settings-btn")
	updateBtn.SetTooltipText("Update hostname")
	updateBtn.ConnectClicked(func() {
		newHostname := n.hostnameEntry.Text()
		if err := n.manager.SetHostname(newHostname); err != nil {
			log.Printf("[CONTROLPANEL] failed to set hostname: %v", err)
			gtkutil.ErrorDialog(nil, "Error", fmt.Sprintf("Failed to set hostname: %v", err))
		} else {
			log.Printf("[CONTROLPANEL] hostname updated to %s", newHostname)
		}
	})

	row.Append(hostnameField)
	row.Append(updateBtn)
	card.Append(row)
	section.Append(card)

	return section
}

func (n *nmConfigProvider) buildDevicesSection() gtk.Widgetter {
	section := gtk.NewBox(gtk.OrientationVertical, 12)
	section.AddCSSClass("settings-section")
	section.SetMarginTop(24)

	subtitle := gtk.NewLabel("Network Devices")
	subtitle.AddCSSClass("settings-subtitle")
	subtitle.SetHAlign(gtk.AlignStart)
	section.Append(subtitle)

	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("system-controls")

	n.devicesList = gtk.NewBox(gtk.OrientationVertical, 0)
	n.devicesKL = gtkutil.NewKeyedList(n.devicesList, true,
		func(dev state.NMDevice) gtk.Widgetter {
			return n.buildDeviceRow(dev)
		},
		nil,
	)
	n.refreshDevicesList()

	card.Append(n.devicesList)
	section.Append(card)

	return section
}

func (n *nmConfigProvider) refreshDevicesList() {
	devices := n.manager.GetDevices()

	if len(devices) == 0 {
		// Clear keyed list and show empty state.
		n.devicesKL.Update(nil)
		emptyLabel := gtk.NewLabel("No network devices found")
		emptyLabel.AddCSSClass("settings-empty-label")
		emptyLabel.SetMarginTop(16)
		emptyLabel.SetMarginBottom(16)
		n.devicesList.Append(emptyLabel)
		return
	}

	sortNMDevices(devices)
	n.devicesKL.Update(devices)
}

func sortNMDevices(devices []state.NMDevice) {
	for i := range devices {
		for j := i + 1; j < len(devices); j++ {
			if nmDeviceRank(devices[j]) < nmDeviceRank(devices[i]) {
				devices[i], devices[j] = devices[j], devices[i]
			}
		}
	}
}

func nmDeviceRank(d state.NMDevice) int {
	if d.ActiveConnection != "" {
		return 0
	}
	if d.State == 30 {
		return 2
	}
	return 1
}

func (n *nmConfigProvider) buildDeviceRow(dev state.NMDevice) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.AddCSSClass("conn-row")
	row.SetMarginStart(16)
	row.SetMarginEnd(16)
	row.SetMarginTop(12)
	row.SetMarginBottom(12)

	icon := gtkutil.MaterialIcon(n.deviceIcon(dev.Type))
	icon.AddCSSClass("conn-row-icon")

	infoBox := gtk.NewBox(gtk.OrientationVertical, 4)
	infoBox.SetHExpand(true)

	nameLabel := gtk.NewLabel(dev.Interface)
	nameLabel.AddCSSClass("conn-row-label")
	nameLabel.SetHAlign(gtk.AlignStart)

	hwAddr := dev.HwAddress
	if hwAddr == "" {
		hwAddr = "No MAC address"
	}

	statusText := deviceStateText(dev.State)
	detailLabel := gtk.NewLabel(fmt.Sprintf("%s  %s  %s", n.deviceTypeLabel(dev.Type), hwAddr, statusText))
	detailLabel.AddCSSClass("conn-row-status")
	detailLabel.SetHAlign(gtk.AlignStart)

	infoBox.Append(nameLabel)
	infoBox.Append(detailLabel)

	// Add connect/disconnect button if device has a connection
	if dev.ActiveConnection != "" {
		disconnectBtn := gtkutil.M3IconButton("link_off", "settings-btn-small")
		disconnectBtn.SetTooltipText("Disconnect")
		disconnectBtn.ConnectClicked(func() {
			if err := n.manager.DeactivateConnection(dev.ActiveConnection); err != nil {
				log.Printf("[CONTROLPANEL] failed to disconnect: %v", err)
				gtkutil.ErrorDialog(nil, "Error", fmt.Sprintf("Failed to disconnect: %v", err))
			}
			// Refresh after a moment
			glib.TimeoutAdd(1000, func() bool {
				n.refreshDevicesList()
				return false
			})
		})
		row.Append(icon)
		row.Append(infoBox)
		row.Append(disconnectBtn)
	} else if dev.State == 30 { // Disconnected - can connect
		connectBtn := gtkutil.M3IconButton("link", "settings-btn-small")
		connectBtn.SetTooltipText("Connect")
		connectBtn.ConnectClicked(func() {
			// For WiFi, show network selector
			if dev.Type == 2 {
				n.showWiFiSelector(dev.Path)
			} else {
				// For other devices, try to auto-connect
				gtkutil.ErrorDialog(nil, "Not Implemented", "Auto-connect for this device type not yet implemented")
			}
		})
		row.Append(icon)
		row.Append(infoBox)
		row.Append(connectBtn)
	} else {
		row.Append(icon)
		row.Append(infoBox)
	}

	return row
}

func (n *nmConfigProvider) buildWiFiSection() gtk.Widgetter {
	section := gtk.NewBox(gtk.OrientationVertical, 12)
	section.AddCSSClass("settings-section")
	section.SetMarginTop(24)

	subtitle := gtk.NewLabel("Available Wi-Fi Networks")
	subtitle.AddCSSClass("settings-subtitle")
	subtitle.SetHAlign(gtk.AlignStart)
	section.Append(subtitle)

	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("system-controls")

	// Scan button row
	scanRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	scanRow.SetMarginStart(16)
	scanRow.SetMarginEnd(16)
	scanRow.SetMarginTop(12)
	scanRow.SetMarginBottom(12)

	scanLabel := gtk.NewLabel("Scan for available networks")
	scanLabel.AddCSSClass("conn-row-label")
	scanLabel.SetHExpand(true)
	scanLabel.SetHAlign(gtk.AlignStart)

	scanBtn := gtkutil.M3IconButton("refresh", "settings-btn")
	scanBtn.SetTooltipText("Scan for Wi-Fi networks")
	scanBtn.ConnectClicked(func() {
		scanLabel.SetText("Scanning...")
		go func() {
			_, err := n.manager.ScanWiFi()
			glib.IdleAdd(func() {
				if err != nil {
					log.Printf("[CONTROLPANEL] WiFi scan failed: %v", err)
					scanLabel.SetText("Scan failed")
				} else {
					n.refreshWiFiList()
					scanLabel.SetText("Scan for available networks")
				}
			})
		}()
	})

	scanRow.Append(scanLabel)
	scanRow.Append(scanBtn)
	card.Append(scanRow)

	// WiFi networks list
	n.wifiList = gtk.NewBox(gtk.OrientationVertical, 0)
	n.wifiKL = gtkutil.NewKeyedList(n.wifiList, true,
		func(net state.WiFiNetwork) gtk.Widgetter {
			return n.buildWiFiRow(net)
		},
		nil,
	)
	card.Append(gtkutil.M3Divider())
	card.Append(n.wifiList)

	section.Append(card)
	return section
}

func (n *nmConfigProvider) refreshWiFiList() {
	networks := n.manager.GetWiFiNetworks()
	sortCPWiFi(networks)
	n.wifiKL.Update(networks)
}

func (n *nmConfigProvider) buildWiFiRow(net state.WiFiNetwork) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.AddCSSClass("conn-row")
	row.SetMarginStart(16)
	row.SetMarginEnd(16)
	row.SetMarginTop(12)
	row.SetMarginBottom(12)

	// Signal strength icon
	signalIcon := "signal_wifi_0_bar"
	if net.Signal > 75 {
		signalIcon = "signal_wifi_4_bar"
	} else if net.Signal > 50 {
		signalIcon = "network_wifi_3_bar"
	} else if net.Signal > 25 {
		signalIcon = "network_wifi_2_bar"
	} else if net.Signal > 0 {
		signalIcon = "wifi_1_bar"
	}

	icon := gtkutil.MaterialIcon(signalIcon)
	icon.AddCSSClass("conn-row-icon")

	infoBox := gtk.NewBox(gtk.OrientationVertical, 4)
	infoBox.SetHExpand(true)

	nameLabel := gtk.NewLabel(net.SSID)
	nameLabel.AddCSSClass("conn-row-label")
	nameLabel.SetHAlign(gtk.AlignStart)

	securityText := "Open"
	if net.Security != "" {
		securityText = net.Security
	}
	detail := fmt.Sprintf("%s  Signal: %d%%", securityText, net.Signal)
	if net.Connected {
		detail += "  Connected"
	} else if net.Saved {
		detail += "  Saved"
	}
	detailLabel := gtk.NewLabel(detail)
	detailLabel.AddCSSClass("conn-row-status")
	detailLabel.SetHAlign(gtk.AlignStart)

	infoBox.Append(nameLabel)
	infoBox.Append(detailLabel)

	// Action button
	row.Append(icon)
	row.Append(infoBox)
	if net.Connected {
		disconnectBtn := gtkutil.M3IconButton("link_off", "settings-btn-small")
		disconnectBtn.SetTooltipText("Disconnect")
		disconnectBtn.ConnectClicked(func() {
			devices := n.manager.GetDevices()
			for _, dev := range devices {
				if dev.ActiveSSID == net.SSID && dev.ActiveConnection != "" {
					n.manager.DeactivateConnection(dev.ActiveConnection)
					break
				}
			}
		})
		row.Append(disconnectBtn)
	} else if net.Saved {
		connectBtn := gtkutil.M3IconButton("link", "settings-btn-small")
		connectBtn.SetTooltipText("Connect")
		connectBtn.ConnectClicked(func() {
			if err := n.manager.ConnectWiFi(net.SSID); err != nil {
				log.Printf("[CONTROLPANEL] failed to connect: %v", err)
				gtkutil.ErrorDialog(nil, "Error", fmt.Sprintf("Failed to connect: %v", err))
			}
		})
		row.Append(connectBtn)
	} else {
		connectBtn := gtkutil.M3IconButton("add", "settings-btn-small")
		connectBtn.SetTooltipText("Connect (requires password)")
		connectBtn.ConnectClicked(func() { n.showWiFiPasswordDialog(net.SSID) })
		row.Append(connectBtn)
	}

	return row
}

func (n *nmConfigProvider) buildConnectionsSection() gtk.Widgetter {
	section := gtk.NewBox(gtk.OrientationVertical, 12)
	section.AddCSSClass("settings-section")
	section.SetMarginTop(24)

	subtitle := gtk.NewLabel("Saved Connections")
	subtitle.AddCSSClass("settings-subtitle")
	subtitle.SetHAlign(gtk.AlignStart)
	section.Append(subtitle)

	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	header.SetMarginBottom(8)
	header.SetHAlign(gtk.AlignEnd)

	refreshBtn := gtkutil.M3IconButton("refresh", "settings-btn")
	refreshBtn.SetTooltipText("Refresh connections list")
	refreshBtn.ConnectClicked(func() { n.refreshConnectionsList() })

	addBtn := gtkutil.M3IconButton("add", "settings-btn")
	addBtn.SetTooltipText("Add new connection")
	addBtn.ConnectClicked(func() { n.showAddConnectionDialog() })

	header.Append(refreshBtn)
	header.Append(addBtn)
	section.Append(header)

	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("system-controls")

	n.connectionsList = gtk.NewBox(gtk.OrientationVertical, 0)
	n.connKL = gtkutil.NewKeyedList(n.connectionsList, true,
		func(conn state.NMConnection) gtk.Widgetter {
			return n.buildConnectionRow(conn)
		},
		nil,
	)
	n.refreshConnectionsList()

	card.Append(n.connectionsList)
	section.Append(card)

	return section
}

func (n *nmConfigProvider) refreshConnectionsList() {
	connections := n.manager.GetConnections()
	sortCPConnections(connections)
	n.connKL.Update(connections)
}

func (n *nmConfigProvider) buildConnectionRow(conn state.NMConnection) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationVertical, 0)
	row.SetMarginStart(16)
	row.SetMarginEnd(16)
	row.SetMarginTop(12)
	row.SetMarginBottom(12)

	// Main row with icon and info
	mainRow := gtk.NewBox(gtk.OrientationHorizontal, 12)

	icon := gtkutil.MaterialIcon(connectionIcon(conn.Type))
	icon.AddCSSClass("conn-row-icon")

	infoBox := gtk.NewBox(gtk.OrientationVertical, 4)
	infoBox.SetHExpand(true)

	nameLabel := gtk.NewLabel(conn.Name)
	nameLabel.AddCSSClass("conn-row-label")
	nameLabel.SetHAlign(gtk.AlignStart)

	detail := conn.TypeLabel
	if conn.IsPrimary {
		detail += "  (Active Gateway)"
	} else if conn.Active {
		detail += "  (Connected)"
	}
	if conn.Secured {
		detail += "  Secured"
	}
	if conn.IPv4Method != "" {
		detail += fmt.Sprintf("  IPv4: %s", conn.IPv4Method)
	}
	detailLabel := gtk.NewLabel(detail)
	detailLabel.AddCSSClass("conn-row-status")
	detailLabel.SetHAlign(gtk.AlignStart)

	infoBox.Append(nameLabel)
	infoBox.Append(detailLabel)

	mainRow.Append(icon)
	mainRow.Append(infoBox)

	// Quick actions for active connection
	if conn.Active {
		disconnectBtn := gtkutil.M3IconButton("link_off", "settings-btn-small")
		disconnectBtn.SetTooltipText("Disconnect")
		disconnectBtn.ConnectClicked(func() {
			if err := n.manager.DeactivateConnection(conn.Path); err != nil {
				log.Printf("[CONTROLPANEL] failed to disconnect: %v", err)
				gtkutil.ErrorDialog(nil, "Error", fmt.Sprintf("Failed to disconnect: %v", err))
			}
			glib.TimeoutAdd(1000, func() bool {
				n.refreshConnectionsList()
				return false
			})
		})
		mainRow.Append(disconnectBtn)
	} else {
		connectBtn := gtkutil.M3IconButton("link", "settings-btn-small")
		connectBtn.SetTooltipText("Connect")
		connectBtn.ConnectClicked(func() {
			// Find suitable device
			devices := n.manager.GetDevices()
			var devicePath string
			for _, dev := range devices {
				// Match connection type to device type - simplified
				if (conn.Type == "802-11-wireless" && dev.Type == 2) ||
					(conn.Type == "802-3-ethernet" && dev.Type == 1) {
					devicePath = dev.Path
					break
				}
			}
			if devicePath != "" {
				if err := n.manager.ActivateConnection(conn.Path, devicePath); err != nil {
					log.Printf("[CONTROLPANEL] failed to connect: %v", err)
					gtkutil.ErrorDialog(nil, "Error", fmt.Sprintf("Failed to connect: %v", err))
				}
			} else {
				gtkutil.ErrorDialog(nil, "Error", "No suitable device found for this connection")
			}
		})
		mainRow.Append(connectBtn)
	}

	row.Append(mainRow)

	// Actions row
	actionsRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	actionsRow.SetMarginStart(36)
	actionsRow.SetMarginTop(8)

	// Autoconnect toggle with label - Material 3 style
	autoBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
	autoLabel := gtk.NewLabel("Auto-connect")
	autoLabel.AddCSSClass("settings-small-label")

	autoconnectSwitch := gtkutil.M3Switch()
	autoconnectSwitch.SetActive(conn.Autoconnect)
	autoconnectSwitch.ConnectStateSet(func(state bool) bool {
		if err := n.manager.SetAutoconnect(conn.Path, state); err != nil {
			log.Printf("[CONTROLPANEL] failed to set autoconnect: %v", err)
		}
		return true
	})

	autoBox.Append(autoLabel)
	autoBox.Append(autoconnectSwitch)
	actionsRow.Append(autoBox)

	actionsRow.Append(gtk.NewBox(gtk.OrientationHorizontal, 0)) // Spacer

	editBtn := gtkutil.M3IconButton("edit", "settings-btn-small")
	editBtn.SetTooltipText("Edit connection")
	editBtn.ConnectClicked(func() { n.showEditConnectionDialog(conn) })
	actionsRow.Append(editBtn)

	deleteBtn := gtkutil.M3IconButton("delete", "settings-btn-small")
	deleteBtn.SetTooltipText("Delete connection")
	deleteBtn.ConnectClicked(func() { n.showDeleteConfirmDialog(conn) })
	actionsRow.Append(deleteBtn)

	row.Append(actionsRow)

	return row
}

// Helper functions

func (n *nmConfigProvider) deviceIcon(deviceType uint32) string {
	switch deviceType {
	case 1:
		return "cable"
	case 2:
		return "wifi"
	case 5:
		return "bluetooth"
	case 14:
		return "settings_ethernet"
	case 30:
		return "vpn_key"
	default:
		return "settings_ethernet"
	}
}

func (n *nmConfigProvider) deviceTypeLabel(deviceType uint32) string {
	switch deviceType {
	case 1:
		return "Wired"
	case 2:
		return "Wi-Fi"
	case 5:
		return "Bluetooth"
	case 14:
		return "Generic"
	case 30:
		return "WireGuard"
	default:
		return "Other"
	}
}

func deviceStateText(state uint32) string {
	switch state {
	case 100:
		return "Connected"
	case 50, 60:
		return "Connecting..."
	case 30:
		return "Disconnected"
	case 20:
		return "Unavailable"
	case 10:
		return "Unmanaged"
	default:
		return "Unknown"
	}
}

func connectionIcon(connType string) string {
	switch connType {
	case "802-11-wireless":
		return "wifi"
	case "802-3-ethernet":
		return "cable"
	case "vpn":
		return "vpn_key"
	case "wireguard":
		return "security"
	case "pppoe":
		return "settings_ethernet"
	case "gsm", "cdma":
		return "signal_cellular_alt"
	default:
		return "settings_ethernet"
	}
}

// Dialog functions

func (n *nmConfigProvider) showWiFiSelector(devicePath string) {
	dialog := gtk.NewDialog()
	dialog.SetTitle("Select Wi-Fi Network")
	dialog.SetTransientFor(nil)
	dialog.SetModal(true)
	dialog.SetDefaultSize(400, 300)

	content := dialog.ContentArea()
	content.SetMarginTop(12)
	content.SetMarginBottom(12)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)

	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)

	list := gtk.NewBox(gtk.OrientationVertical, 8)
	networks := n.manager.GetWiFiNetworks()

	for _, net := range networks {
		if net.Saved {
			row := gtk.NewBox(gtk.OrientationHorizontal, 8)
			row.AddCSSClass("conn-row")

			label := gtk.NewLabel(net.SSID)
			label.SetHExpand(true)
			label.SetHAlign(gtk.AlignStart)

			btn := gtk.NewButtonWithLabel("Connect")
			btn.AddCSSClass("m3-filled-btn")
			btn.ConnectClicked(func() {
				// Find connection for this SSID
				connections := n.manager.GetConnections()
				for _, conn := range connections {
					if conn.SSID == net.SSID {
						n.manager.ActivateConnection(conn.Path, devicePath)
						break
					}
				}
				dialog.Close()
			})

			row.Append(label)
			row.Append(btn)
			list.Append(row)
		}
	}

	scroll.SetChild(list)
	content.Append(scroll)

	cancelBtn := gtk.NewButtonWithLabel("Cancel")
	cancelBtn.ConnectClicked(func() {
		dialog.Close()
	})
	dialog.AddActionWidget(cancelBtn, int(gtk.ResponseCancel))

	dialog.Show()
}

func (n *nmConfigProvider) showWiFiPasswordDialog(ssid string) {
	dialog := gtk.NewDialog()
	dialog.SetTitle(fmt.Sprintf("Connect to %s", ssid))
	dialog.SetTransientFor(nil)
	dialog.SetModal(true)
	dialog.SetDefaultSize(400, 200)
	dialog.AddCSSClass("popup-panel")

	content := dialog.ContentArea()
	content.AddCSSClass("settings-stack")
	content.SetMarginTop(24)
	content.SetMarginBottom(24)
	content.SetMarginStart(24)
	content.SetMarginEnd(24)

	title := gtk.NewLabel(fmt.Sprintf("Connect to %s", ssid))
	title.AddCSSClass("settings-title")
	title.SetHAlign(gtk.AlignStart)
	content.Append(title)

	// Password - Material 3 Outlined Password Field
	passBox := gtk.NewBox(gtk.OrientationVertical, 8)
	passBox.SetMarginTop(16)
	passLabel := gtk.NewLabel("Password")
	passLabel.AddCSSClass("settings-small-label")
	passLabel.SetHAlign(gtk.AlignStart)
	passBox.Append(passLabel)
	passField := n.createPasswordField()
	passEntry := passField.Entry()
	passBox.Append(passField)
	content.Append(passBox)

	buttonBox := gtk.NewBox(gtk.OrientationHorizontal, 12)
	buttonBox.SetHAlign(gtk.AlignEnd)
	buttonBox.SetMarginTop(24)

	cancelBtn := gtk.NewButtonWithLabel("Cancel")
	cancelBtn.AddCSSClass("m3-text-btn")
	cancelBtn.ConnectClicked(func() {
		dialog.Close()
	})
	buttonBox.Append(cancelBtn)

	connectBtn := gtk.NewButtonWithLabel("Connect")
	connectBtn.AddCSSClass("m3-filled-btn")
	connectBtn.ConnectClicked(func() {
		password := passEntry.Text()
		if err := n.manager.ConnectWiFiWithPassword(ssid, password); err != nil {
			log.Printf("[CONTROLPANEL] failed to connect: %v", err)
			gtkutil.ErrorDialog(nil, "Connection Failed", err.Error())
		}
		dialog.Close()
	})
	buttonBox.Append(connectBtn)

	content.Append(buttonBox)
	dialog.Show()
}

func (n *nmConfigProvider) showDeleteConfirmDialog(conn state.NMConnection) {
	dialog := gtk.NewDialog()
	dialog.SetTitle("Confirm Delete")
	dialog.SetTransientFor(nil)
	dialog.SetModal(true)
	dialog.AddCSSClass("popup-panel")

	content := dialog.ContentArea()
	content.AddCSSClass("settings-stack")
	content.SetMarginTop(24)
	content.SetMarginBottom(24)
	content.SetMarginStart(24)
	content.SetMarginEnd(24)

	label := gtk.NewLabel(fmt.Sprintf("Delete connection '%s'?", conn.Name))
	label.AddCSSClass("settings-subtitle")
	sublabel := gtk.NewLabel("This action cannot be undone.")
	sublabel.AddCSSClass("settings-small-label")
	content.Append(label)
	content.Append(sublabel)

	cancelBtn := gtk.NewButtonWithLabel("Cancel")
	cancelBtn.AddCSSClass("m3-text-btn")
	cancelBtn.ConnectClicked(func() {
		dialog.Close()
	})
	content.Append(cancelBtn)

	deleteBtn := gtk.NewButtonWithLabel("Delete")
	deleteBtn.AddCSSClass("m3-filled-btn")
	deleteBtn.ConnectClicked(func() {
		if err := n.manager.DeleteConnection(conn.Path); err != nil {
			log.Printf("[CONTROLPANEL] failed to delete connection: %v", err)
			gtkutil.ErrorDialog(nil, "Error", fmt.Sprintf("Failed to delete: %v", err))
		} else {
			n.refreshConnectionsList()
			dialog.Close()
		}
	})
	content.Append(deleteBtn)

	dialog.Show()
}

// createPasswordField creates a password field using the new M3 component
func (n *nmConfigProvider) createPasswordField() *gtkutil.M3OutlinedTextField {
	return gtkutil.NewM3OutlinedPasswordField()
}

func (n *nmConfigProvider) showEditConnectionDialog(conn state.NMConnection) {
	dialog := gtk.NewDialog()
	dialog.SetTitle(fmt.Sprintf("Edit %s", conn.Name))
	dialog.SetTransientFor(nil)
	dialog.SetModal(true)
	dialog.SetDefaultSize(450, 500)
	dialog.AddCSSClass("popup-panel")

	content := dialog.ContentArea()
	content.AddCSSClass("settings-stack")
	content.SetMarginTop(24)
	content.SetMarginBottom(24)
	content.SetMarginStart(24)
	content.SetMarginEnd(24)

	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)

	box := gtk.NewBox(gtk.OrientationVertical, 16)
	box.AddCSSClass("settings-page")

	// Title
	title := gtk.NewLabel("Edit Connection")
	title.AddCSSClass("settings-title")
	title.SetHAlign(gtk.AlignStart)
	box.Append(title)

	// Connection name - Material 3 Outlined Text Field style
	nameBox := gtk.NewBox(gtk.OrientationVertical, 8)
	nameBox.SetMarginTop(16)
	nameLabel := gtk.NewLabel("Connection Name")
	nameLabel.AddCSSClass("settings-small-label")
	nameLabel.SetHAlign(gtk.AlignStart)
	nameBox.Append(nameLabel)
	nameField := gtkutil.NewM3OutlinedTextField()
	nameField.SetText(conn.Name)
	nameEntry := nameField.Entry()
	nameBox.Append(nameField)
	box.Append(nameBox)

	// Connection type (read-only chip style)
	typeBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
	typeBox.SetMarginTop(8)
	typeLabel := gtk.NewLabel("Type")
	typeLabel.AddCSSClass("settings-small-label")
	typeChip := gtk.NewLabel(conn.TypeLabel)
	typeChip.AddCSSClass("m3-assist-chip")
	typeBox.Append(typeLabel)
	typeBox.Append(typeChip)
	box.Append(typeBox)

	// IPv4 Method - Material 3 Dropdown
	ipv4Box := gtk.NewBox(gtk.OrientationVertical, 8)
	ipv4Box.SetMarginTop(16)
	ipv4Label := gtk.NewLabel("IPv4 Method")
	ipv4Label.AddCSSClass("settings-small-label")
	ipv4Label.SetHAlign(gtk.AlignStart)
	ipv4Dropdown := gtkutil.NewM3Dropdown([]string{"Automatic (DHCP)", "Manual", "Disabled"})
	if conn.IPv4Method == "manual" {
		ipv4Dropdown.SetSelected(1)
	} else if conn.IPv4Method == "disabled" {
		ipv4Dropdown.SetSelected(2)
	}
	ipv4Box.Append(ipv4Label)
	ipv4Box.Append(ipv4Dropdown)
	box.Append(ipv4Box)

	// Manual IP settings (initially hidden if not manual)
	manualBox := gtk.NewBox(gtk.OrientationVertical, 16)
	manualBox.SetVisible(conn.IPv4Method == "manual")

	addressField := gtkutil.NewM3OutlinedTextField()
	addressField.Entry().SetPlaceholderText("IP Address (e.g. 192.168.1.10/24)")
	addressField.SetText(conn.IPv4Address)
	manualBox.Append(gtkutil.SettingsSection("Address", addressField))

	gatewayField := gtkutil.NewM3OutlinedTextField()
	gatewayField.Entry().SetPlaceholderText("Gateway (e.g. 192.168.1.1)")
	gatewayField.SetText(conn.IPv4Gateway)
	manualBox.Append(gtkutil.SettingsSection("Gateway", gatewayField))

	dnsField := gtkutil.NewM3OutlinedTextField()
	dnsField.Entry().SetPlaceholderText("DNS Servers (comma separated)")
	dnsField.SetText(strings.Join(conn.IPv4DNS, ", "))
	manualBox.Append(gtkutil.SettingsSection("DNS Servers", dnsField))

	box.Append(manualBox)

	ipv4Dropdown.ConnectSelected(func(idx int) {
		manualBox.SetVisible(idx == 1)
	})

	// Autoconnect - Material 3 Switch with row
	autoRow := gtk.NewBox(gtk.OrientationHorizontal, 0)
	autoRow.AddCSSClass("m3-switch-row")
	autoRow.SetMarginTop(16)
	autoLabel := gtk.NewLabel("Auto-connect")
	autoLabel.AddCSSClass("settings-subtitle")
	autoLabel.SetHExpand(true)
	autoLabel.SetHAlign(gtk.AlignStart)
	autoSwitch := gtkutil.M3Switch()
	autoSwitch.SetActive(conn.Autoconnect)
	autoRow.Append(autoLabel)
	autoRow.Append(autoSwitch)
	box.Append(autoRow)

	scroll.SetChild(box)
	content.Append(scroll)

	// Button row
	buttonBox := gtk.NewBox(gtk.OrientationHorizontal, 12)
	buttonBox.SetHAlign(gtk.AlignEnd)
	buttonBox.SetMarginTop(16)

	cancelBtn := gtk.NewButtonWithLabel("Cancel")
	cancelBtn.AddCSSClass("m3-text-btn")
	cancelBtn.ConnectClicked(func() {
		dialog.Close()
	})
	buttonBox.Append(cancelBtn)

	saveBtn := gtk.NewButtonWithLabel("Save")
	saveBtn.AddCSSClass("m3-filled-btn")
	saveBtn.ConnectClicked(func() {
		// Get the updated name
		newName := nameEntry.Text()

		method := "auto"
		switch ipv4Dropdown.Selected() {
		case 1:
			method = "manual"
		case 2:
			method = "disabled"
		}

		ipv4Settings := map[string]dbus.Variant{
			"method": dbus.MakeVariant(method),
		}

		if method == "manual" {
			addrParts := strings.Split(addressField.Entry().Text(), "/")
			if len(addrParts) == 2 {
				ipv4Settings["address-data"] = dbus.MakeVariant([]map[string]dbus.Variant{
					{
						"address": dbus.MakeVariant(addrParts[0]),
						"prefix":  dbus.MakeVariant(func() uint32 { var p uint32; fmt.Sscanf(addrParts[1], "%d", &p); return p }()),
					},
				})
			}
			if gw := gatewayField.Entry().Text(); gw != "" {
				ipv4Settings["gateway"] = dbus.MakeVariant(gw)
			}
			if dnsStr := dnsField.Entry().Text(); dnsStr != "" {
				dnsList := strings.Split(dnsStr, ",")
				dnsUints := []uint32{}
				for _, d := range dnsList {
					var a, b, c, d_byte byte
					if n, _ := fmt.Sscanf(strings.TrimSpace(d), "%d.%d.%d.%d", &a, &b, &c, &d_byte); n == 4 {
						dnsUints = append(dnsUints, uint32(a)<<24|uint32(b)<<16|uint32(c)<<8|uint32(d_byte))
					}
				}
				ipv4Settings["dns"] = dbus.MakeVariant(dnsUints)
			}
		}

		settings := map[string]map[string]dbus.Variant{
			"connection": {
				"id":          dbus.MakeVariant(newName),
				"autoconnect": dbus.MakeVariant(autoSwitch.Active()),
			},
			"ipv4": ipv4Settings,
		}

		if err := n.manager.UpdateConnection(conn.Path, settings); err != nil {
			log.Printf("[CONTROLPANEL] failed to update connection: %v", err)
			gtkutil.ErrorDialog(nil, "Error", fmt.Sprintf("Failed to save: %v", err))
		} else {
			n.refreshConnectionsList()
			dialog.Close()
		}
	})
	buttonBox.Append(saveBtn)

	content.Append(buttonBox)
	dialog.Show()
}


func (n *nmConfigProvider) showAddConnectionDialog() {
	dialog := gtk.NewDialog()
	dialog.SetTitle("Add New Connection")
	dialog.SetTransientFor(nil)
	dialog.SetModal(true)
	dialog.SetDefaultSize(400, 300)

	content := dialog.ContentArea()
	content.SetMarginTop(12)
	content.SetMarginBottom(12)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)

	// Connection type selector
	typeLabel := gtk.NewLabel("Select connection type:")
	typeLabel.SetHAlign(gtk.AlignStart)
	content.Append(typeLabel)

	types := []string{"Wi-Fi", "Ethernet", "WireGuard"}
	typeList := gtk.NewBox(gtk.OrientationVertical, 8)
	typeList.SetMarginTop(12)

	for _, connType := range types {
		btn := gtk.NewButtonWithLabel(connType)
		btn.AddCSSClass("m3-text-btn")
		btn.SetHAlign(gtk.AlignStart)
		btn.ConnectClicked(func(t string) func() {
			return func() {
				dialog.Close()
				n.showAddConnectionForm(t)
			}
		}(connType))
		typeList.Append(btn)
	}

	content.Append(typeList)

	cancelBtn := gtk.NewButtonWithLabel("Cancel")
	cancelBtn.ConnectClicked(func() {
		dialog.Close()
	})
	dialog.AddActionWidget(cancelBtn, int(gtk.ResponseCancel))

	dialog.Show()
}

func (n *nmConfigProvider) showAddConnectionForm(connType string) {
	switch connType {
	case "Wi-Fi":
		n.showAddWiFiDialog()
	case "Ethernet":
		n.showAddEthernetDialog()
	case "WireGuard":
		n.showAddWireGuardDialog()
	}
}

func (n *nmConfigProvider) showAddWiFiDialog() {
	dialog := gtk.NewDialog()
	dialog.SetTitle("Add Wi-Fi Connection")
	dialog.SetTransientFor(nil)
	dialog.SetModal(true)
	dialog.SetDefaultSize(400, 350)
	dialog.AddCSSClass("popup-panel")

	content := dialog.ContentArea()
	content.AddCSSClass("settings-stack")
	content.SetMarginTop(24)
	content.SetMarginBottom(24)
	content.SetMarginStart(24)
	content.SetMarginEnd(24)

	title := gtk.NewLabel("Add Wi-Fi Connection")
	title.AddCSSClass("settings-title")
	title.SetHAlign(gtk.AlignStart)
	content.Append(title)

	// SSID - Material 3 Outlined Text Field
	ssidBox := gtk.NewBox(gtk.OrientationVertical, 8)
	ssidBox.SetMarginTop(16)
	ssidLabel := gtk.NewLabel("Network Name (SSID)")
	ssidLabel.AddCSSClass("settings-small-label")
	ssidLabel.SetHAlign(gtk.AlignStart)
	ssidBox.Append(ssidLabel)
	ssidField := gtkutil.NewM3OutlinedTextField()
	ssidEntry := ssidField.Entry()
	ssidBox.Append(ssidField)
	content.Append(ssidBox)

	// Password - Material 3 Outlined Password Field
	passBox := gtk.NewBox(gtk.OrientationVertical, 8)
	passBox.SetMarginTop(16)
	passLabel := gtk.NewLabel("Password")
	passLabel.AddCSSClass("settings-small-label")
	passLabel.SetHAlign(gtk.AlignStart)
	passBox.Append(passLabel)
	passField := n.createPasswordField()
	passEntry := passField.Entry()
	passBox.Append(passField)
	content.Append(passBox)

	// Auto-connect - Material 3 Switch
	autoRow := gtk.NewBox(gtk.OrientationHorizontal, 0)
	autoRow.AddCSSClass("m3-switch-row")
	autoRow.SetMarginTop(16)
	autoLabel := gtk.NewLabel("Auto-connect")
	autoLabel.AddCSSClass("settings-subtitle")
	autoLabel.SetHExpand(true)
	autoLabel.SetHAlign(gtk.AlignStart)
	autoSwitch := gtkutil.M3Switch()
	autoSwitch.SetActive(true)
	autoRow.Append(autoLabel)
	autoRow.Append(autoSwitch)
	content.Append(autoRow)

	// Button row
	buttonBox := gtk.NewBox(gtk.OrientationHorizontal, 12)
	buttonBox.SetHAlign(gtk.AlignEnd)
	buttonBox.SetMarginTop(24)

	cancelBtn := gtk.NewButtonWithLabel("Cancel")
	cancelBtn.AddCSSClass("m3-text-btn")
	cancelBtn.ConnectClicked(func() {
		dialog.Close()
	})
	buttonBox.Append(cancelBtn)

	addBtn := gtk.NewButtonWithLabel("Add")
	addBtn.AddCSSClass("m3-filled-btn")
	addBtn.ConnectClicked(func() {
		ssid := ssidEntry.Text()
		password := passEntry.Text()
		autoconnect := autoSwitch.Active()

		if ssid == "" {
			gtkutil.ErrorDialog(nil, "Error", "Please enter a network name")
			return
		}

		settings := map[string]map[string]dbus.Variant{
			"connection": {
				"id":          dbus.MakeVariant(ssid),
				"type":        dbus.MakeVariant("802-11-wireless"),
				"autoconnect": dbus.MakeVariant(autoconnect),
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
			settings["802-11-wireless-security"] = map[string]dbus.Variant{
				"key-mgmt": dbus.MakeVariant("wpa-psk"),
				"psk":      dbus.MakeVariant(password),
			}
		}

		if _, err := n.manager.AddConnection(settings); err != nil {
			log.Printf("[CONTROLPANEL] failed to add connection: %v", err)
			gtkutil.ErrorDialog(nil, "Error", fmt.Sprintf("Failed to add connection: %v", err))
		} else {
			n.refreshConnectionsList()
			dialog.Close()
		}
	})
	buttonBox.Append(addBtn)

	content.Append(buttonBox)
	dialog.Show()
}

func (n *nmConfigProvider) showAddEthernetDialog() {
	dialog := gtk.NewDialog()
	dialog.SetTitle("Add Ethernet Connection")
	dialog.SetTransientFor(nil)
	dialog.SetModal(true)
	dialog.SetDefaultSize(400, 280)
	dialog.AddCSSClass("popup-panel")

	content := dialog.ContentArea()
	content.AddCSSClass("settings-stack")
	content.SetMarginTop(24)
	content.SetMarginBottom(24)
	content.SetMarginStart(24)
	content.SetMarginEnd(24)

	title := gtk.NewLabel("Add Ethernet Connection")
	title.AddCSSClass("settings-title")
	title.SetHAlign(gtk.AlignStart)
	content.Append(title)

	// Connection name - Material 3 Outlined Text Field
	nameBox := gtk.NewBox(gtk.OrientationVertical, 8)
	nameBox.SetMarginTop(16)
	nameLabel := gtk.NewLabel("Connection Name")
	nameLabel.AddCSSClass("settings-small-label")
	nameLabel.SetHAlign(gtk.AlignStart)
	nameBox.Append(nameLabel)
	nameField := gtkutil.NewM3OutlinedTextField()
	nameEntry := nameField.Entry()
	nameBox.Append(nameField)
	content.Append(nameBox)

	// Auto-connect - Material 3 Switch
	autoRow := gtk.NewBox(gtk.OrientationHorizontal, 0)
	autoRow.AddCSSClass("m3-switch-row")
	autoRow.SetMarginTop(16)
	autoLabel := gtk.NewLabel("Auto-connect")
	autoLabel.AddCSSClass("settings-subtitle")
	autoLabel.SetHExpand(true)
	autoLabel.SetHAlign(gtk.AlignStart)
	autoSwitch := gtkutil.M3Switch()
	autoSwitch.SetActive(true)
	autoRow.Append(autoLabel)
	autoRow.Append(autoSwitch)
	content.Append(autoRow)

	// Button row
	buttonBox := gtk.NewBox(gtk.OrientationHorizontal, 12)
	buttonBox.SetHAlign(gtk.AlignEnd)
	buttonBox.SetMarginTop(24)

	cancelBtn := gtk.NewButtonWithLabel("Cancel")
	cancelBtn.AddCSSClass("m3-text-btn")
	cancelBtn.ConnectClicked(func() {
		dialog.Close()
	})
	buttonBox.Append(cancelBtn)

	addBtn := gtk.NewButtonWithLabel("Add")
	addBtn.AddCSSClass("m3-filled-btn")
	addBtn.ConnectClicked(func() {
		name := nameEntry.Text()
		autoconnect := autoSwitch.Active()

		if name == "" {
			gtkutil.ErrorDialog(nil, "Error", "Please enter a connection name")
			return
		}

		settings := map[string]map[string]dbus.Variant{
			"connection": {
				"id":          dbus.MakeVariant(name),
				"type":        dbus.MakeVariant("802-3-ethernet"),
				"autoconnect": dbus.MakeVariant(autoconnect),
			},
			"ipv4": {
				"method": dbus.MakeVariant("auto"),
			},
			"ipv6": {
				"method": dbus.MakeVariant("auto"),
			},
		}

		if _, err := n.manager.AddConnection(settings); err != nil {
			log.Printf("[CONTROLPANEL] failed to add connection: %v", err)
			gtkutil.ErrorDialog(nil, "Error", fmt.Sprintf("Failed to add connection: %v", err))
		} else {
			n.refreshConnectionsList()
			dialog.Close()
		}
	})
	buttonBox.Append(addBtn)

	content.Append(buttonBox)
	dialog.Show()
}

func (n *nmConfigProvider) showAddWireGuardDialog() {
	dialog := gtk.NewDialog()
	dialog.SetTitle("Add WireGuard Connection")
	dialog.SetTransientFor(nil)
	dialog.SetModal(true)
	dialog.SetDefaultSize(400, 280)
	dialog.AddCSSClass("popup-panel")

	content := dialog.ContentArea()
	content.AddCSSClass("settings-stack")
	content.SetMarginTop(24)
	content.SetMarginBottom(24)
	content.SetMarginStart(24)
	content.SetMarginEnd(24)

	title := gtk.NewLabel("Add WireGuard Connection")
	title.AddCSSClass("settings-title")
	title.SetHAlign(gtk.AlignStart)
	content.Append(title)

	nameField := gtkutil.NewM3OutlinedTextField()
	nameField.Entry().SetPlaceholderText("Connection Name (e.g. MyVPN)")
	content.Append(gtkutil.SettingsSection("Name", nameField))

	ifaceField := gtkutil.NewM3OutlinedTextField()
	ifaceField.Entry().SetPlaceholderText("Interface (e.g. wg0)")
	content.Append(gtkutil.SettingsSection("Interface", ifaceField))

	buttonBox := gtk.NewBox(gtk.OrientationHorizontal, 12)
	buttonBox.SetHAlign(gtk.AlignEnd)
	buttonBox.SetMarginTop(24)

	cancelBtn := gtk.NewButtonWithLabel("Cancel")
	cancelBtn.AddCSSClass("m3-text-btn")
	cancelBtn.ConnectClicked(func() { dialog.Close() })
	buttonBox.Append(cancelBtn)

	addBtn := gtk.NewButtonWithLabel("Add")
	addBtn.AddCSSClass("m3-filled-btn")
	addBtn.ConnectClicked(func() {
		name := nameField.Entry().Text()
		iface := ifaceField.Entry().Text()

		if name == "" || iface == "" {
			gtkutil.ErrorDialog(nil, "Error", "Please fill in all fields")
			return
		}

		settings := map[string]map[string]dbus.Variant{
			"connection": {
				"id":             dbus.MakeVariant(name),
				"type":           dbus.MakeVariant("wireguard"),
				"interface-name": dbus.MakeVariant(iface),
			},
			"wireguard": {}, // Minimal WG section
		}

		if _, err := n.manager.AddConnection(settings); err != nil {
			log.Printf("[CONTROLPANEL] failed to add WG: %v", err)
			gtkutil.ErrorDialog(nil, "Error", err.Error())
		} else {
			n.refreshConnectionsList()
			dialog.Close()
		}
	})
	buttonBox.Append(addBtn)
	content.Append(buttonBox)
	dialog.Show()
}

// monitorSignals listens for NM D-Bus property changes and refreshes the UI.
func (n *nmConfigProvider) monitorSignals() {
	ch := make(chan *dbus.Signal, 16)
	n.conn.Signal(ch)
	defer n.conn.RemoveSignal(ch)

	busObj := n.conn.BusObject()
	_ = busObj.Call("org.freedesktop.DBus.AddMatch", 0,
		"type='signal',sender='org.freedesktop.NetworkManager',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged'").Err

	for range ch {
		// Drain queued signals before handling.
		drain:
		for {
			select {
			case <-ch:
			default:
				break drain
			}
		}
		glib.IdleAdd(func() {
			n.refreshDevicesList()
			n.refreshWiFiList()
			n.refreshConnectionsList()
		})
	}
}

func sortCPWiFi(nets []state.WiFiNetwork) {
	for i := range nets {
		for j := i + 1; j < len(nets); j++ {
			if cpWiFiRank(nets[j]) < cpWiFiRank(nets[i]) {
				nets[i], nets[j] = nets[j], nets[i]
			}
		}
	}
}

func cpWiFiRank(n state.WiFiNetwork) int {
	if n.Connected {
		return 0
	}
	if n.Saved {
		return 1
	}
	return 2
}

func sortCPConnections(conns []state.NMConnection) {
	for i := range conns {
		for j := i + 1; j < len(conns); j++ {
			if cpConnRank(conns[j]) < cpConnRank(conns[i]) {
				conns[i], conns[j] = conns[j], conns[i]
			}
		}
	}
}

func cpConnRank(c state.NMConnection) int {
	if c.IsPrimary {
		return 0
	}
	if c.Active {
		return 1
	}
	return 2
}
