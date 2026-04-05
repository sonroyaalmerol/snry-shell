package widgets

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// NewWiFiWidget creates an Android 16-style WiFi panel with toggle, collapsible
// sections, and confirmation dialogs.
func NewWiFiWidget(b *bus.Bus, refs *servicerefs.ServiceRefs, parent *gtk.ApplicationWindow) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("conn-widget")

	// Networks list (collapsible).
	listBox := gtk.NewBox(gtk.OrientationVertical, 0)
	listBox.AddCSSClass("conn-list")

	revealer := gtk.NewRevealer()
	revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	revealer.SetTransitionDuration(250)
	revealer.SetRevealChild(true)
	revealer.SetChild(listBox)

	// Section header.
	sectionHeader := gtkutil.SectionHeader("Available networks", 0, revealer, func() {
		if refs.Network != nil {
			go refs.Network.ScanWiFi()
		}
	})
	box.Append(sectionHeader)

	box.Append(revealer)

	// Scan button.
	scanBtn := gtkutil.MaterialButtonWithClass("refresh", "conn-scan-btn")
	scanBtn.ConnectClicked(func() {
		if refs.Network != nil {
			go refs.Network.ScanWiFi()
		}
	})
	scanBtnWrapper := gtk.NewBox(gtk.OrientationHorizontal, 0)
	scanBtnWrapper.SetHAlign(gtk.AlignEnd)
	scanBtnWrapper.Append(scanBtn)
	box.Append(scanBtnWrapper)

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
			gtkutil.ClearChildren(&listBox.Widget, listBox.Remove)

			for _, net := range networks {
				row := newWiFiRow(parent, refs, net)
				listBox.Append(row)
			}

			gtkutil.UpdateSectionHeader(sectionHeader, len(networks))
		})
	})

	return box
}

func newWiFiRow(parent *gtk.ApplicationWindow, refs *servicerefs.ServiceRefs, net state.WiFiNetwork) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.AddCSSClass("conn-row")
	if net.Connected {
		row.AddCSSClass("conn-row-connected")
	}

	signalIcon := gtkutil.MaterialIcon(signalStrengthIcon(net.Signal))
	signalIcon.AddCSSClass("conn-row-icon")

	ssidLabel := gtk.NewLabel(net.SSID)
	ssidLabel.AddCSSClass("conn-row-label")

	meta := gtk.NewBox(gtk.OrientationHorizontal, 4)
	meta.AddCSSClass("conn-row-meta")

	if net.Security != "" {
		secIcon := gtkutil.MaterialIcon("lock")
		secIcon.AddCSSClass("conn-row-meta-icon")
		meta.Append(secIcon)
	}

	if net.Connected {
		check := gtkutil.MaterialIcon("check_circle")
		check.AddCSSClass("conn-row-connected-icon")
		meta.Append(check)
	}

	row.Append(signalIcon)
	row.Append(ssidLabel)
	row.Append(meta)

	if refs.Network != nil {
		ssid := net.SSID
		saved := net.Saved
		security := net.Security
		connected := net.Connected
		click := gtk.NewGestureClick()
		click.SetButton(1)
		click.ConnectPressed(func(_ int, _ float64, _ float64) {
			click.SetState(gtk.EventSequenceClaimed)
		})
		click.ConnectReleased(func(_ int, _ float64, _ float64) {
			switch {
			case connected:
				gtkutil.ActionDialog(
					parent,
					"Connected to network",
					ssid,
					[]gtkutil.ActionDialogAction{
						{Label: "Disconnect", OnClick: func() { go refs.Network.DisconnectWiFi() }},
						{Label: "Forget", CSSClass: "m3-dialog-btn-error", OnClick: func() { go refs.Network.ForgetWiFi(ssid) }},
					},
				)
			case saved:
				go refs.Network.ConnectWiFi(ssid)
			case security != "":
				gtkutil.PasswordDialog(
					parent,
					"Connect to network",
					ssid,
					"Password",
					func(password string) {
						go refs.Network.ConnectWithPassword(ssid, password)
					},
				)
			default:
				go refs.Network.ConnectWithPassword(ssid, "")
			}
		})
		row.AddController(click)
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
