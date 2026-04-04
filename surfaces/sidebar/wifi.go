package sidebar

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// newWiFiWidget creates a WiFi network list widget.
func newWiFiWidget(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.AddCSSClass("wifi-widget")

	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	headerLabel := gtk.NewLabel("WiFi Networks")
	headerLabel.AddCSSClass("notif-group-header")
	headerLabel.SetHExpand(true)

	scanBtn := gtkutil.MaterialButtonWithClass("refresh", "wifi-scan-btn")
	scanBtn.ConnectClicked(func() {
		if refs.Network != nil {
			go refs.Network.ScanWiFi()
		}
	})

	header.Append(headerLabel)
	header.Append(scanBtn)
	box.Append(header)

	listBox := gtk.NewListBox()
	listBox.AddCSSClass("wifi-list")
	listBox.SetSelectionMode(gtk.SelectionNone)

	// Scan on creation.
	if refs.Network != nil {
		go refs.Network.ScanWiFi()
	}

	b.Subscribe(bus.TopicWiFiNetworks, func(e bus.Event) {
		networks, ok := e.Data.([]state.WiFiNetwork)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			gtkutil.ClearChildren(&listBox.Widget)

			for _, net := range networks {
				row := newWiFiRow(b, refs, net)
				listBox.Append(row)
			}
		})
	})

	box.Append(listBox)
	return box
}

func newWiFiRow(b *bus.Bus, refs *servicerefs.ServiceRefs, net state.WiFiNetwork) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 8)
	row.AddCSSClass("wifi-row")

	icon := gtk.NewLabel("wifi")
	icon.AddCSSClass("material-icon")
	icon.AddCSSClass("wifi-icon")
	if net.Connected {
		row.AddCSSClass("wifi-connected")
	}

	ssidLabel := gtk.NewLabel(net.SSID)
	ssidLabel.AddCSSClass("wifi-ssid")
	ssidLabel.SetHExpand(true)
	ssidLabel.SetHAlign(gtk.AlignStart)

	signalLabel := gtk.NewLabel(signalStrengthIcon(net.Signal))
	signalLabel.AddCSSClass("material-icon")
	signalLabel.AddCSSClass("wifi-signal")

	securityLabel := gtk.NewLabel("")
	if net.Security != "" {
		securityLabel.SetText("lock")
	}
	securityLabel.AddCSSClass("material-icon")
	securityLabel.AddCSSClass("wifi-security")

	statusLabel := gtk.NewLabel("")
	if net.Connected {
		statusLabel.SetText("Connected")
		statusLabel.AddCSSClass("wifi-status-connected")
	}

	row.Append(icon)
	row.Append(ssidLabel)
	row.Append(signalLabel)
	row.Append(securityLabel)
	row.Append(statusLabel)

	if !net.Connected && refs.Network != nil {
		connectBtn := gtkutil.MaterialButtonWithClass("login", "wifi-connect-btn")

		ssid := net.SSID
		connectBtn.ConnectClicked(func() {
			go refs.Network.ConnectWiFi(ssid)
		})
		row.Append(connectBtn)
	}

	return row
}

func signalStrengthIcon(signal int) string {
	switch {
	case signal >= 80:
		return "signal_wifi_4_bar"
	case signal >= 60:
		return "signal_wifi_3_bar"
	case signal >= 40:
		return "signal_wifi_2_bar"
	case signal >= 20:
		return "signal_wifi_1_bar"
	default:
		return "signal_wifi_0_bar"
	}
}
