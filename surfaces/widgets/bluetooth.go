package widgets

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// NewBluetoothWidget creates an Android 16-style Bluetooth panel with a
// single flat device list and confirmation dialogs.
func NewBluetoothWidget(b *bus.Bus, refs *servicerefs.ServiceRefs, parent *gtk.ApplicationWindow) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("conn-widget")

	// Power toggle row.
	btSwitch := gtkutil.M3Switch()
	settingBT := false
	btSwitch.ConnectStateSet(func(state bool) bool {
		if settingBT {
			return false
		}
		if refs.Bluetooth != nil {
			go refs.Bluetooth.SetPowered(state)
		}
		return true
	})

	b.Subscribe(bus.TopicBluetooth, func(e bus.Event) {
		bs, ok := e.Data.(state.BluetoothState)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			settingBT = true
			btSwitch.SetActive(bs.Powered)
			settingBT = false
		})
	})

	box.Append(gtkutil.SwitchRow("Bluetooth", btSwitch))

	rescan := func() {
		if refs.Bluetooth != nil {
			go func() {
				_ = refs.Bluetooth.StartScan()
				time.Sleep(5 * time.Second)
				_, _ = refs.Bluetooth.GetDevices()
			}()
		}
	}

	// Single device list.
	listBox := gtk.NewBox(gtk.OrientationVertical, 0)
	listBox.AddCSSClass("conn-list")

	revealer := gtk.NewRevealer()
	revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	revealer.SetTransitionDuration(250)
	revealer.SetRevealChild(true)
	revealer.SetChild(listBox)

	sectionHeader := gtkutil.SectionHeader("Devices", 0, revealer, rescan)
	box.Append(sectionHeader)
	box.Append(revealer)

	// Scan button.
	scanBtn := gtkutil.M3IconButton("search", "conn-scan-btn")

	restoreScanBtn := func() {
		scanBtn.SetSensitive(true)
		scanBtn.SetChild(gtkutil.MaterialIcon("search"))
	}

	scanBtn.ConnectClicked(func() {
		scanBtn.SetSensitive(false)
		scanBtn.SetChild(gtkutil.M3Spinner())
		rescan()
	})
	scanBtnWrapper := gtk.NewBox(gtk.OrientationHorizontal, 0)
	scanBtnWrapper.SetHAlign(gtk.AlignEnd)
	scanBtnWrapper.Append(scanBtn)
	box.Append(scanBtnWrapper)

	if refs.Bluetooth != nil {
		go rescan()
	}

	// Keyed device list for diff-based updates (no flickering).
	btKL := gtkutil.NewKeyedList(listBox, false,
		func(dev state.BluetoothDevice) gtk.Widgetter {
			return newBTDeviceRow(parent, refs, dev, rescan)
		},
		nil,
	)

	b.Subscribe(bus.TopicBluetoothDevices, func(e bus.Event) {
		devices, ok := e.Data.([]state.BluetoothDevice)
		if !ok {
			return
		}
		sorted := make([]state.BluetoothDevice, len(devices))
		copy(sorted, devices)
		sortDevices(sorted)
		glib.IdleAdd(func() {
			btKL.Update(sorted)
			gtkutil.UpdateSectionHeader(sectionHeader, len(devices))
			restoreScanBtn()
		})
	})

	return box
}

func sortDevices(devices []state.BluetoothDevice) {
	for i := range devices {
		for j := i + 1; j < len(devices); j++ {
			if deviceRank(devices[j]) < deviceRank(devices[i]) {
				devices[i], devices[j] = devices[j], devices[i]
			}
		}
	}
}

func deviceRank(d state.BluetoothDevice) int {
	if d.Connected {
		return 0
	}
	if d.Paired {
		return 1
	}
	return 2
}

func newBTDeviceRow(parent *gtk.ApplicationWindow, refs *servicerefs.ServiceRefs, dev state.BluetoothDevice, rescan func()) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.AddCSSClass("conn-row")
	row.SetCursorFromName("pointer")
	if dev.Connected {
		row.AddCSSClass("conn-row-connected")
	}

	devIcon := gtkutil.MaterialIcon("bluetooth")
	devIcon.AddCSSClass("conn-row-icon")

	name := dev.Name
	if name == "" {
		name = dev.Address
	}
	nameLabel := gtk.NewLabel(name)
	nameLabel.AddCSSClass("conn-row-label")

	meta := gtk.NewBox(gtk.OrientationHorizontal, 4)
	meta.AddCSSClass("conn-row-meta")

	if dev.Connected {
		statusLabel := gtk.NewLabel("Connected")
		statusLabel.AddCSSClass("conn-row-status")
		meta.Append(statusLabel)
	} else if dev.Paired {
		statusLabel := gtk.NewLabel("Paired")
		statusLabel.AddCSSClass("conn-row-status")
		meta.Append(statusLabel)
	}

	row.Append(devIcon)
	row.Append(nameLabel)
	row.Append(meta)

	if refs.Bluetooth != nil {
		addr := dev.Address

		setLoading := func() {
			row.AddCSSClass("conn-row-loading")
			row.SetSensitive(false)
			gtkutil.ClearChildren(&meta.Widget, meta.Remove)
			meta.Append(gtkutil.M3Spinner())
		}

		gtkutil.ClaimedClick(&row.Widget, func() {
			switch {
			case dev.Connected:
				gtkutil.ConfirmDialog(
					parent,
					"Disconnect device",
					name,
					"Disconnect",
					func() {
						setLoading()
						go func() {
							if err := refs.Bluetooth.DisconnectDevice(addr); err != nil {
								glib.IdleAdd(func() { gtkutil.ErrorDialog(parent, "Disconnect failed", err.Error()) })
							}
							rescan()
						}()
					},
				)
			case dev.Paired:
				gtkutil.ConfirmDialog(
					parent,
					"Connect to device",
					name,
					"Connect",
					func() {
						setLoading()
						go func() {
							if err := refs.Bluetooth.ConnectDevice(addr); err != nil {
								glib.IdleAdd(func() { gtkutil.ErrorDialog(parent, "Connection failed", err.Error()) })
							}
							rescan()
						}()
					},
				)
			default:
				gtkutil.ConfirmDialog(
					parent,
					"Pair with device",
					name,
					"Pair",
					func() {
						setLoading()
						go func() {
							if err := refs.Bluetooth.PairDevice(addr); err != nil {
								glib.IdleAdd(func() { gtkutil.ErrorDialog(parent, "Pairing failed", err.Error()) })
							}
							rescan()
						}()
					},
				)
			}
		})
	}

	return row
}
