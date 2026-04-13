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

	// Networks list (collapsible).
	listBox := gtk.NewBox(gtk.OrientationVertical, 0)
	listBox.AddCSSClass("conn-list")

	revealer := gtk.NewRevealer()
	revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	revealer.SetTransitionDuration(250)
	revealer.SetRevealChild(true)
	revealer.SetChild(listBox)

	// Section header.
	sectionHeader := gtkutil.SectionHeader("Available networks", 0, revealer, nil)
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

	// Row updaters keyed by SSID — allows updateFn to refresh visual state.
	rowUpdaters := make(map[string]func(state.WiFiNetwork))

	// Keyed WiFi list for diff-based updates (no flickering).
	wifiKL := gtkutil.NewKeyedList(listBox, false,
		func(net state.WiFiNetwork) gtk.Widgetter {
			w, update := newWiFiRow(parent, refs, net)
			rowUpdaters[net.SSID] = update
			return w
		},
		func(net state.WiFiNetwork, _ gtk.Widgetter) {
			if update, ok := rowUpdaters[net.SSID]; ok {
				update(net)
			}
		},
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

// newWiFiRow creates a WiFi list row and returns the widget plus an update function.
// The update function refreshes the Connected visual state in-place without rebuilding.
func newWiFiRow(parent *gtk.ApplicationWindow, refs *servicerefs.ServiceRefs, net state.WiFiNetwork) (gtk.Widgetter, func(state.WiFiNetwork)) {
	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.AddCSSClass("conn-row")
	row.SetCursorFromName("pointer")

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

	// Mutable connected state — shared between click handler and updateFn.
	connected := net.Connected

	// Connected indicator widgets (added/removed by updateFn).
	var checkIcon *gtk.Label
	var badge *gtk.Label

	setConnected := func(isConnected bool) {
		connected = isConnected

		// Undo loading state if present.
		row.RemoveCSSClass("conn-row-loading")
		row.SetSensitive(true)

		// Clear meta to remove any spinner from setLoading,
		// then rebuild indicators from scratch.
		gtkutil.ClearChildren(&meta.Widget, meta.Remove)
		checkIcon = nil
		badge = nil

		if isConnected {
			row.AddCSSClass("conn-row-connected")
			checkIcon = gtkutil.MaterialIcon("check_circle")
			checkIcon.AddCSSClass("conn-row-connected-icon")
			meta.Append(checkIcon)
			badge = gtk.NewLabel("ACTIVE")
			badge.AddCSSClass("m3-assist-chip")
			badge.SetVAlign(gtk.AlignCenter)
			meta.Append(badge)
		} else {
			row.RemoveCSSClass("conn-row-connected")
		}

		// Re-add security icon if present.
		if net.Security != "" {
			secIcon := gtkutil.MaterialIcon("lock")
			secIcon.AddCSSClass("conn-row-meta-icon")
			meta.Append(secIcon)
		}
	}

	// Apply initial state.
	setConnected(net.Connected)

	row.Append(signalIcon)
	row.Append(ssidLabel)
	row.Append(meta)

	if refs.Network != nil {
		ssid := net.SSID
		saved := net.Saved
		security := net.Security

		setLoading := func() {
			row.AddCSSClass("conn-row-loading")
			row.SetSensitive(false)
			gtkutil.ClearChildren(&meta.Widget, meta.Remove)
			meta.Append(gtkutil.M3Spinner())
			checkIcon = nil
			badge = nil
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
							}()
						}},
						{Label: "Forget", CSSClass: "m3-dialog-btn-error", OnClick: func() {
							setLoading()
							go func() {
								if err := refs.Network.ForgetWiFi(ssid); err != nil {
									glib.IdleAdd(func() { gtkutil.ErrorDialog(parent, "Forget failed", err.Error()) })
								}
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
						}()
					},
				)
			default:
				setLoading()
				go func() {
					if err := refs.Network.ConnectWithPassword(ssid, ""); err != nil {
						glib.IdleAdd(func() { gtkutil.ErrorDialog(parent, "Connection failed", err.Error()) })
					}
				}()
			}
		})
	}

	// updateFn refreshes the row's visual state in-place.
	update := func(net state.WiFiNetwork) {
		setConnected(net.Connected)
	}

	return row, update
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
