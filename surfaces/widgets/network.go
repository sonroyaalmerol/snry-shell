package widgets

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// NewNetworkWidget creates an Android 16-style Network panel with Ethernet info,
// WiFi toggle, collapsible sections, and confirmation dialogs.
func NewNetworkWidget(b *bus.Bus, refs *servicerefs.ServiceRefs, parent *gtk.ApplicationWindow) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("conn-widget")

	// Ethernet section
	ethBox := gtk.NewBox(gtk.OrientationVertical, 0)
	ethBox.SetVisible(false) // Only show if ethernet is connected

	ethLabel := gtk.NewLabel("Ethernet")
	ethLabel.AddCSSClass("conn-section-header")
	ethLabel.SetHAlign(gtk.AlignStart)
	ethBox.Append(ethLabel)

	ethRow := gtk.NewBox(gtk.OrientationHorizontal, 12)
	ethRow.AddCSSClass("conn-row")
	ethRow.AddCSSClass("conn-row-connected")

	ethIcon := gtkutil.MaterialIcon("settings_ethernet")
	ethIcon.AddCSSClass("conn-row-icon")
	ethRow.Append(ethIcon)

	ethInfo := gtk.NewBox(gtk.OrientationVertical, 2)
	ethName := gtk.NewLabel("")
	ethName.AddCSSClass("conn-row-label")
	ethName.SetHAlign(gtk.AlignStart)
	ethIP := gtk.NewLabel("")
	ethIP.AddCSSClass("conn-row-meta-text")
	ethIP.SetHAlign(gtk.AlignStart)
	ethInfo.Append(ethName)
	ethInfo.Append(ethIP)
	ethRow.Append(ethInfo)

	ethCheck := gtkutil.MaterialIcon("check_circle")
	ethCheck.AddCSSClass("conn-row-connected-icon")
	ethCheck.SetHAlign(gtk.AlignEnd)
	ethCheck.SetHExpand(true)
	ethRow.Append(ethCheck)

	ethBox.Append(ethRow)
	box.Append(ethBox)

	// WiFi toggle row.
	wifiSwitch := gtkutil.M3Switch()
	settingWifi := false
	wifiSwitch.ConnectStateSet(func(state bool) bool {
		if settingWifi {
			return false
		}
		if refs.Network != nil {
			go refs.Network.SetWiFi(state)
		}
		return true
	})

	box.Append(gtkutil.SwitchRow("WiFi", wifiSwitch))

	rescan := func() {
		if refs.Network != nil {
			go refs.Network.ScanWiFi()
		}
	}

	// Networks list (collapsible).
	listBox := gtk.NewBox(gtk.OrientationVertical, 0)
	listBox.AddCSSClass("conn-list")

	revealer := gtk.NewRevealer()
	revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	revealer.SetTransitionDuration(250)
	revealer.SetRevealChild(true)
	revealer.SetChild(listBox)

	// Section header.
	sectionHeader := gtkutil.SectionHeader("Available networks", 0, revealer, rescan)
	box.Append(sectionHeader)

	box.Append(revealer)

	// Scan button.
	scanBtn := gtkutil.M3IconButton("refresh", "conn-scan-btn")

	restoreScanBtn := func() {
		scanBtn.SetSensitive(true)
		scanBtn.SetChild(gtkutil.MaterialIcon("refresh"))
	}

	scanBtn.ConnectClicked(func() {
		if refs.Network == nil {
			return
		}
		scanBtn.SetSensitive(false)
		scanBtn.SetChild(gtkutil.M3Spinner())
		go refs.Network.ScanWiFi()
	})
	scanBtnWrapper := gtk.NewBox(gtk.OrientationHorizontal, 0)
	scanBtnWrapper.SetHAlign(gtk.AlignEnd)
	scanBtnWrapper.Append(scanBtn)
	box.Append(scanBtnWrapper)

	// Scan on creation.
	if refs.Network != nil {
		go refs.Network.ScanWiFi()
	}

	// Keyed WiFi list for diff-based updates (no flickering).
	wifiKL := gtkutil.NewKeyedList(listBox, false,
		func(net state.WiFiNetwork) gtk.Widgetter {
			return newWiFiRow(parent, refs, net, rescan)
		},
		nil,
	)

	// Unified network state handler: WiFi switch, ethernet info, and WiFi list.
	b.Subscribe(bus.TopicNetwork, func(e bus.Event) {
		ns, ok := e.Data.(state.NetworkState)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			settingWifi = true
			wifiSwitch.SetActive(ns.WirelessEnabled)
			settingWifi = false

			if ns.Type == "ethernet" && ns.Connected {
				ethName.SetText(ns.ActiveConnectionName)
				ipText := ns.IPv4
				if ns.IPv6 != "" {
					if ipText != "" {
						ipText += " | "
					}
					ipText += ns.IPv6
				}
				ethIP.SetText(ipText)
				ethBox.SetVisible(true)
			} else {
				ethBox.SetVisible(false)
			}

			// Update WiFi list from unified state.
			sorted := make([]state.WiFiNetwork, len(ns.WiFiNetworks))
			copy(sorted, ns.WiFiNetworks)
			sortWiFiNetworks(sorted)
			wifiKL.Update(sorted)
			gtkutil.UpdateSectionHeader(sectionHeader, len(sorted))
			restoreScanBtn()
		})
	})

	return box
}

func sortWiFiNetworks(nets []state.WiFiNetwork) {
	for i := range nets {
		for j := i + 1; j < len(nets); j++ {
			if wifiRank(nets[j]) < wifiRank(nets[i]) {
				nets[i], nets[j] = nets[j], nets[i]
			}
		}
	}
}

func wifiRank(n state.WiFiNetwork) int {
	if n.Connected {
		return 0
	}
	if n.Saved {
		return 1
	}
	return 2
}

func newWiFiRow(parent *gtk.ApplicationWindow, refs *servicerefs.ServiceRefs, net state.WiFiNetwork, rescan func()) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.AddCSSClass("conn-row")
	row.SetCursorFromName("pointer")
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

		badge := gtk.NewLabel("ACTIVE")
		badge.AddCSSClass("m3-assist-chip") // Reuse chip style
		badge.SetVAlign(gtk.AlignCenter)
		meta.Append(badge)
	}

	row.Append(signalIcon)
	row.Append(ssidLabel)
	row.Append(meta)

	if refs.Network != nil {
		ssid := net.SSID
		saved := net.Saved
		security := net.Security
		connected := net.Connected

		setLoading := func() {
			row.AddCSSClass("conn-row-loading")
			row.SetSensitive(false)
			gtkutil.ClearChildren(&meta.Widget, meta.Remove)
			meta.Append(gtkutil.M3Spinner())
		}

		gtkutil.ClaimedClick(&row.Widget, func() {
			switch {
			case connected:
				gtkutil.ActionDialog(
					parent,
					"Connected to network",
					ssid,
					[]gtkutil.ActionDialogAction{
						{Label: "Disconnect", OnClick: func() {
							setLoading()
							go func() {
								if err := refs.Network.DisconnectWiFi(); err != nil {
									glib.IdleAdd(func() { gtkutil.ErrorDialog(parent, "Disconnect failed", err.Error()) })
								}
								rescan()
							}()
						}},
						{Label: "Forget", CSSClass: "m3-dialog-btn-error", OnClick: func() {
							setLoading()
							go func() {
								if err := refs.Network.ForgetWiFi(ssid); err != nil {
									glib.IdleAdd(func() { gtkutil.ErrorDialog(parent, "Forget failed", err.Error()) })
								}
								rescan()
							}()
						}},
					},
				)
			case saved:
				setLoading()
				go func() {
					if err := refs.Network.ConnectWiFi(ssid); err != nil {
						glib.IdleAdd(func() { gtkutil.ErrorDialog(parent, "Connection failed", err.Error()) })
					}
					rescan()
				}()
			case security != "":
				gtkutil.PasswordDialog(
					parent,
					"Connect to network",
					ssid,
					func(password string) {
						setLoading()
						go func() {
							if err := refs.Network.ConnectWithPassword(ssid, password); err != nil {
								glib.IdleAdd(func() { gtkutil.ErrorDialog(parent, "Connection failed", err.Error()) })
							}
							rescan()
						}()
					},
				)
			default:
				setLoading()
				go func() {
					if err := refs.Network.ConnectWithPassword(ssid, ""); err != nil {
						glib.IdleAdd(func() { gtkutil.ErrorDialog(parent, "Connection failed", err.Error()) })
					}
					rescan()
				}()
			}
		})
	}

	return row
}

func signalStrengthIcon(signal int) string {
	switch {
	case signal >= 60:
		return "network_wifi_3_bar"
	case signal >= 30:
		return "network_wifi_2_bar"
	default:
		return "wifi_1_bar"
	}
}
