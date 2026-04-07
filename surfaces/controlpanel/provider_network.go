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
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Printf("[CONTROLPANEL] cannot connect to system bus for NetworkManager: %v", err)
		return nil
	}

	nmSvc := network.New(conn, nil)
	return &nmConfigProvider{nmService: nmSvc}
}

func (n *nmConfigProvider) Name() string {
	return "Network"
}

func (n *nmConfigProvider) Icon() string {
	return "network_ethernet"
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

	box.Append(n.buildHostnameSection())
	box.Append(n.buildDevicesSection())
	box.Append(n.buildConnectionsSection())

	scroll.SetChild(box)
	return scroll
}

func (n *nmConfigProvider) buildHostnameSection() gtk.Widgetter {
	section := gtk.NewBox(gtk.OrientationVertical, 12)
	section.AddCSSClass("settings-page")

	title := gtk.NewLabel("System Hostname")
	title.AddCSSClass("settings-label")
	title.SetHAlign(gtk.AlignStart)
	section.Append(title)

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
		if n.nmService != nil {
			newHostname := hostnameEntry.Text()
			if err := n.nmService.SetHostname(newHostname); err != nil {
				log.Printf("[CONTROLPANEL] failed to set hostname: %v", err)
			} else {
				log.Printf("[CONTROLPANEL] hostname updated to %s", newHostname)
			}
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
	section.AddCSSClass("settings-page")
	section.SetMarginTop(24)

	title := gtk.NewLabel("Network Devices")
	title.AddCSSClass("settings-label")
	title.SetHAlign(gtk.AlignStart)
	section.Append(title)

	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("system-controls")

	if n.nmService != nil {
		devices, err := n.nmService.GetDevices()
		if err != nil {
			log.Printf("[CONTROLPANEL] failed to get devices: %v", err)
		}

		for i, dev := range devices {
			if i > 0 {
				card.Append(gtkutil.M3Divider())
			}
			row := n.buildDeviceRow(dev)
			card.Append(row)
		}
	}

	section.Append(card)
	return section
}

func (n *nmConfigProvider) buildDeviceRow(dev state.NMDevice) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.AddCSSClass("conn-row")

	icon := gtk.NewLabel("")
	icon.AddCSSClass("conn-row-icon")
	icon.AddCSSClass("material-icon")

	var typeLabel string
	switch dev.Type {
	case 1:
		icon.SetText("ethernet")
		typeLabel = "Wired"
	case 2:
		icon.SetText("wifi")
		typeLabel = "Wi-Fi"
	case 14:
		icon.SetText("bluetooth")
		typeLabel = "Bluetooth"
	default:
		icon.SetText("settings_ethernet")
		typeLabel = "Other"
	}

	infoBox := gtk.NewBox(gtk.OrientationVertical, 2)
	infoBox.SetHExpand(true)

	nameLabel := gtk.NewLabel(dev.Interface)
	nameLabel.AddCSSClass("conn-row-label")
	nameLabel.SetHAlign(gtk.AlignStart)

	hwAddr := dev.HwAddress
	if hwAddr == "" {
		hwAddr = "No address"
	}
	detailLabel := gtk.NewLabel(fmt.Sprintf("%s %s", typeLabel, hwAddr))
	detailLabel.AddCSSClass("conn-row-status")
	detailLabel.SetHAlign(gtk.AlignStart)

	infoBox.Append(nameLabel)
	infoBox.Append(detailLabel)

	statusLabel := gtk.NewLabel(deviceStateText(dev.State))
	statusLabel.AddCSSClass("conn-row-meta-icon")

	row.Append(icon)
	row.Append(infoBox)
	row.Append(statusLabel)

	return row
}

func (n *nmConfigProvider) buildConnectionsSection() gtk.Widgetter {
	section := gtk.NewBox(gtk.OrientationVertical, 12)
	section.AddCSSClass("settings-page")
	section.SetMarginTop(24)

	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	header.SetMarginBottom(8)

	title := gtk.NewLabel("Connections")
	title.AddCSSClass("settings-label")
	title.SetHAlign(gtk.AlignStart)
	title.SetHExpand(true)

	addBtn := gtkutil.M3IconButton("add", "settings-btn")
	addBtn.SetTooltipText("Add new connection")
	addBtn.ConnectClicked(func() {
		log.Printf("[CONTROLPANEL] add connection clicked")
	})

	header.Append(title)
	header.Append(addBtn)
	section.Append(header)

	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("system-controls")

	if n.nmService != nil {
		conns, err := n.nmService.GetAllConnections()
		if err != nil {
			log.Printf("[CONTROLPANEL] failed to get connections: %v", err)
		}

		for i, conn := range conns {
			if i > 0 {
				card.Append(gtkutil.M3Divider())
			}
			row := n.buildConnectionRow(conn)
			card.Append(row)
		}
	}

	section.Append(card)
	return section
}

func (n *nmConfigProvider) buildConnectionRow(conn state.NMConnection) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.AddCSSClass("conn-row")

	icon := gtk.NewLabel(connectionIcon(conn.Type))
	icon.AddCSSClass("conn-row-icon")
	icon.AddCSSClass("material-icon")

	infoBox := gtk.NewBox(gtk.OrientationVertical, 2)
	infoBox.SetHExpand(true)

	nameLabel := gtk.NewLabel(conn.Name)
	nameLabel.AddCSSClass("conn-row-label")
	nameLabel.SetHAlign(gtk.AlignStart)

	detail := conn.TypeLabel
	if conn.Autoconnect {
		detail += " Auto"
	}
	detailLabel := gtk.NewLabel(detail)
	detailLabel.AddCSSClass("conn-row-status")
	detailLabel.SetHAlign(gtk.AlignStart)

	infoBox.Append(nameLabel)
	infoBox.Append(detailLabel)

	actionsBox := gtk.NewBox(gtk.OrientationHorizontal, 4)

	autoconnectSwitch := gtk.NewSwitch()
	autoconnectSwitch.SetActive(conn.Autoconnect)
	autoconnectSwitch.SetTooltipText("Auto-connect")
	autoconnectSwitch.ConnectStateSet(func(state bool) bool {
		if n.nmService != nil {
			if err := n.nmService.SetAutoconnect(conn.Path, state); err != nil {
				log.Printf("[CONTROLPANEL] failed to set autoconnect: %v", err)
			}
		}
		return false
	})

	deleteBtn := gtkutil.M3IconButton("delete", "settings-btn-small")
	deleteBtn.SetTooltipText("Delete connection")
	deleteBtn.ConnectClicked(func() {
		if n.nmService != nil {
			if err := n.nmService.DeleteConnection(conn.Path); err != nil {
				log.Printf("[CONTROLPANEL] failed to delete connection: %v", err)
			}
		}
	})

	actionsBox.Append(autoconnectSwitch)
	actionsBox.Append(deleteBtn)

	row.Append(icon)
	row.Append(infoBox)
	row.Append(actionsBox)

	return row
}

func deviceStateText(state uint32) string {
	switch state {
	case 100:
		return "Activated"
	case 50, 60:
		return "Connecting"
	case 30:
		return "Disconnected"
	case 20:
		return "Unavailable"
	default:
		return "Unknown"
	}
}

func connectionIcon(connType string) string {
	switch connType {
	case "802-11-wireless":
		return "wifi"
	case "802-3-ethernet":
		return "ethernet"
	case "vpn":
		return "vpn_key"
	case "wireguard":
		return "security"
	default:
		return "settings_ethernet"
	}
}
