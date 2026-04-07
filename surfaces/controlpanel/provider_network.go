package controlpanel

import (
	"fmt"
	"log"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/services/network"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// nmConfigProvider implements ConfigProvider for NetworkManager settings
type nmConfigProvider struct {
	nmService *network.Service
}

// newNMProviderWithConnection creates a network provider with its own D-Bus connection
func newNMProviderWithConnection() ConfigProvider {
	log.Printf("[CONTROLPANEL] Attempting to connect to system D-Bus for NetworkManager...")
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Printf("[CONTROLPANEL] cannot connect to system bus for NetworkManager: %v", err)
		return nil
	}
	log.Printf("[CONTROLPANEL] Successfully connected to system D-Bus")

	nmSvc := network.New(conn, nil)

	// Test the connection by getting hostname
	if hostname, err := nmSvc.GetHostname(); err != nil {
		log.Printf("[CONTROLPANEL] Warning: Failed to get hostname, NM may not be available: %v", err)
	} else {
		log.Printf("[CONTROLPANEL] NetworkManager available, hostname: %s", hostname)
	}

	return &nmConfigProvider{nmService: nmSvc}
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

	// Hostname section
	box.Append(n.buildHostnameSection())

	// Devices section
	box.Append(n.buildDevicesSection())

	// WiFi Networks section
	box.Append(n.buildWiFiSection())

	// Connections section
	box.Append(n.buildConnectionsSection())

	scroll.SetChild(box)
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

	hostnameEntry := gtk.NewEntry()
	hostnameEntry.AddCSSClass("settings-entry")
	hostnameEntry.SetHExpand(true)

	if n.nmService != nil {
		if hostname, err := n.nmService.GetHostname(); err == nil {
			hostnameEntry.SetText(hostname)
		}
	}

	updateBtn := gtkutil.M3IconButton("check", "settings-btn")
	updateBtn.SetTooltipText("Update hostname")
	updateBtn.ConnectClicked(func() {
		log.Printf("[CONTROLPANEL] Update hostname button clicked")
		if n.nmService == nil {
			log.Printf("[CONTROLPANEL] ERROR: nmService is nil!")
			return
		}
		newHostname := hostnameEntry.Text()
		log.Printf("[CONTROLPANEL] Setting hostname to: %s", newHostname)
		if err := n.nmService.SetHostname(newHostname); err != nil {
			log.Printf("[CONTROLPANEL] failed to set hostname: %v", err)
		} else {
			log.Printf("[CONTROLPANEL] hostname updated to %s", newHostname)
		}
	})

	row.Append(hostnameEntry)
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

	if n.nmService != nil {
		devices, err := n.nmService.GetDevices()
		if err != nil {
			log.Printf("[CONTROLPANEL] failed to get devices: %v", err)
		}

		if len(devices) == 0 {
			emptyLabel := gtk.NewLabel("No network devices found")
			emptyLabel.AddCSSClass("settings-empty-label")
			emptyLabel.SetMarginTop(16)
			emptyLabel.SetMarginBottom(16)
			card.Append(emptyLabel)
		} else {
			for i, dev := range devices {
				if i > 0 {
					card.Append(gtkutil.M3Divider())
				}
				row := n.buildDeviceRow(dev)
				card.Append(row)
			}
		}
	}

	section.Append(card)
	return section
}

func (n *nmConfigProvider) buildDeviceRow(dev state.NMDevice) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.AddCSSClass("conn-row")
	row.SetMarginStart(16)
	row.SetMarginEnd(16)
	row.SetMarginTop(12)
	row.SetMarginBottom(12)

	// Device icon
	icon := gtk.NewLabel("")
	icon.AddCSSClass("conn-row-icon")
	icon.AddCSSClass("material-icon")

	var typeLabel string
	switch dev.Type {
	case 1:
		icon.SetText("cable")
		typeLabel = "Wired"
	case 2:
		icon.SetText("wifi")
		typeLabel = "Wi-Fi"
	case 14:
		icon.SetText("bluetooth")
		typeLabel = "Bluetooth"
	case 30:
		icon.SetText("vpn_key")
		typeLabel = "WWAN"
	default:
		icon.SetText("settings_ethernet")
		typeLabel = "Other"
	}

	// Info box
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
	detailLabel := gtk.NewLabel(fmt.Sprintf("%s  %s  %s", typeLabel, hwAddr, statusText))
	detailLabel.AddCSSClass("conn-row-status")
	detailLabel.SetHAlign(gtk.AlignStart)

	infoBox.Append(nameLabel)
	infoBox.Append(detailLabel)

	row.Append(icon)
	row.Append(infoBox)

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

	// Scan button
	scanRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	scanRow.SetMarginStart(16)
	scanRow.SetMarginEnd(16)
	scanRow.SetMarginTop(12)
	scanRow.SetMarginBottom(12)

	scanLabel := gtk.NewLabel("Scan for networks")
	scanLabel.AddCSSClass("conn-row-label")
	scanLabel.SetHExpand(true)
	scanLabel.SetHAlign(gtk.AlignStart)

	scanBtn := gtkutil.M3IconButton("refresh", "settings-btn")
	scanBtn.SetTooltipText("Scan for Wi-Fi networks")
	scanBtn.ConnectClicked(func() {
		log.Printf("[CONTROLPANEL] Scan WiFi button clicked")
		if n.nmService == nil {
			log.Printf("[CONTROLPANEL] ERROR: nmService is nil!")
			return
		}
		go func() {
			log.Printf("[CONTROLPANEL] Starting WiFi scan...")
			networks, err := n.nmService.ScanWiFi()
			if err != nil {
				log.Printf("[CONTROLPANEL] WiFi scan failed: %v", err)
			} else {
				log.Printf("[CONTROLPANEL] WiFi scan completed, found %d networks", len(networks))
			}
		}()
	})

	scanRow.Append(scanLabel)
	scanRow.Append(scanBtn)
	card.Append(scanRow)

	section.Append(card)
	return section
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
	refreshBtn.ConnectClicked(func() {
		// TODO: Refresh the view
		log.Printf("[CONTROLPANEL] refresh connections clicked")
	})

	addBtn := gtkutil.M3IconButton("add", "settings-btn")
	addBtn.SetTooltipText("Add new connection")
	addBtn.ConnectClicked(func() {
		// TODO: Open add connection dialog
		log.Printf("[CONTROLPANEL] add connection clicked")
	})

	header.Append(refreshBtn)
	header.Append(addBtn)
	section.Append(header)

	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("system-controls")

	if n.nmService != nil {
		conns, err := n.nmService.GetAllConnections()
		if err != nil {
			log.Printf("[CONTROLPANEL] failed to get connections: %v", err)
		}

		if len(conns) == 0 {
			emptyLabel := gtk.NewLabel("No saved connections")
			emptyLabel.AddCSSClass("settings-empty-label")
			emptyLabel.SetMarginTop(16)
			emptyLabel.SetMarginBottom(16)
			card.Append(emptyLabel)
		} else {
			for i, conn := range conns {
				if i > 0 {
					card.Append(gtkutil.M3Divider())
				}
				row := n.buildConnectionRow(conn)
				card.Append(row)
			}
		}
	}

	section.Append(card)
	return section
}

func (n *nmConfigProvider) buildConnectionRow(conn state.NMConnection) gtk.Widgetter {
	log.Printf("[CONTROLPANEL] Building connection row for: %s", conn.Name)
	row := gtk.NewBox(gtk.OrientationVertical, 0)
	row.SetMarginStart(16)
	row.SetMarginEnd(16)
	row.SetMarginTop(12)
	row.SetMarginBottom(12)

	// Main row with icon and info
	mainRow := gtk.NewBox(gtk.OrientationHorizontal, 12)

	icon := gtk.NewLabel(connectionIcon(conn.Type))
	icon.AddCSSClass("conn-row-icon")
	icon.AddCSSClass("material-icon")

	infoBox := gtk.NewBox(gtk.OrientationVertical, 4)
	infoBox.SetHExpand(true)

	nameLabel := gtk.NewLabel(conn.Name)
	nameLabel.AddCSSClass("conn-row-label")
	nameLabel.SetHAlign(gtk.AlignStart)

	detail := conn.TypeLabel
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

	row.Append(mainRow)

	// Actions row
	actionsRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	actionsRow.SetMarginStart(36)
	actionsRow.SetMarginTop(8)

	// Autoconnect toggle with label
	autoBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
	autoLabel := gtk.NewLabel("Auto-connect")
	autoLabel.AddCSSClass("settings-small-label")

	autoconnectSwitch := gtk.NewSwitch()
	autoconnectSwitch.SetActive(conn.Autoconnect)
	autoconnectSwitch.SetSensitive(true)
	autoconnectSwitch.SetCanTarget(true)
	autoconnectSwitch.ConnectStateSet(func(state bool) bool {
		log.Printf("[CONTROLPANEL] Autoconnect toggle changed for %s: %v", conn.Name, state)
		if n.nmService == nil {
			log.Printf("[CONTROLPANEL] ERROR: nmService is nil!")
			return false
		}
		if err := n.nmService.SetAutoconnect(conn.Path, state); err != nil {
			log.Printf("[CONTROLPANEL] failed to set autoconnect: %v", err)
		} else {
			log.Printf("[CONTROLPANEL] autoconnect set to %v for %s", state, conn.Name)
		}
		return false
	})

	autoBox.Append(autoLabel)
	autoBox.Append(autoconnectSwitch)
	actionsRow.Append(autoBox)

	actionsRow.Append(gtk.NewBox(gtk.OrientationHorizontal, 0)) // Spacer

	// Edit button
	editBtn := gtkutil.M3IconButton("edit", "settings-btn-small")
	editBtn.SetTooltipText("Edit connection")
	editBtn.ConnectClicked(func() {
		log.Printf("[CONTROLPANEL] Edit button clicked for connection: %s (path: %s)", conn.Name, conn.Path)
	})
	actionsRow.Append(editBtn)

	// Delete button - make it more visible/obvious
	deleteBtn := gtk.NewButton()
	deleteBtn.AddCSSClass("m3-icon-btn")
	deleteBtn.AddCSSClass("settings-btn-small")
	deleteBtn.SetTooltipText("Delete connection")
	deleteIcon := gtkutil.MaterialIcon("delete")
	deleteBtn.SetChild(deleteIcon)
	deleteBtn.SetCursorFromName("pointer")
	deleteBtn.SetSensitive(true)
	deleteBtn.ConnectClicked(func() {
		log.Printf("[CONTROLPANEL] Delete button clicked for connection: %s (path: %s)", conn.Name, conn.Path)
		if n.nmService == nil {
			log.Printf("[CONTROLPANEL] ERROR: nmService is nil!")
			return
		}
		log.Printf("[CONTROLPANEL] Deleting connection at path: %s", conn.Path)
		if err := n.nmService.DeleteConnection(conn.Path); err != nil {
			log.Printf("[CONTROLPANEL] failed to delete connection: %v", err)
		} else {
			log.Printf("[CONTROLPANEL] successfully deleted connection: %s", conn.Name)
		}
	})
	actionsRow.Append(deleteBtn)

	row.Append(actionsRow)

	return row
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
