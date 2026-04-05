package widgets

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// NewBluetoothWidget creates an Android 16-style Bluetooth panel with toggle,
// paired/available sections, and confirmation dialogs.
func NewBluetoothWidget(b *bus.Bus, refs *servicerefs.ServiceRefs, parent *gtk.ApplicationWindow) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("conn-widget")

	rescan := func() {
		if refs.Bluetooth != nil {
			go func() {
				_ = refs.Bluetooth.StartScan()
				_, _ = refs.Bluetooth.GetDevices()
			}()
		}
	}

	// Available devices section.
	availableListBox := gtk.NewBox(gtk.OrientationVertical, 0)
	availableListBox.AddCSSClass("conn-list")
	availableRevealer := gtk.NewRevealer()
	availableRevealer.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	availableRevealer.SetTransitionDuration(250)
	availableRevealer.SetRevealChild(true)
	availableRevealer.SetChild(availableListBox)

	availableHeader := gtkutil.SectionHeader("Available devices", 0, availableRevealer, rescan)
	box.Append(availableHeader)
	box.Append(availableRevealer)

	// Paired devices section.
	pairedListBox := gtk.NewBox(gtk.OrientationVertical, 0)
	pairedListBox.AddCSSClass("conn-list")
	pairedRevealer := gtk.NewRevealer()
	pairedRevealer.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	pairedRevealer.SetTransitionDuration(250)
	pairedRevealer.SetRevealChild(true)
	pairedRevealer.SetChild(pairedListBox)

	pairedHeader := gtkutil.SectionHeader("Paired devices", 0, pairedRevealer, nil)
	box.Append(pairedHeader)
	box.Append(pairedRevealer)

	// Scan button.
	scanBtn := gtk.NewButton()
	scanBtn.AddCSSClass("conn-scan-btn")
	scanBtn.SetChild(gtkutil.MaterialIcon("search"))

	restoreScanBtn := func() {
		scanBtn.SetSensitive(true)
		scanBtn.SetChild(gtkutil.MaterialIcon("search"))
	}

	scanBtn.ConnectClicked(func() {
		scanBtn.SetSensitive(false)
		scanBtn.SetChild(gtkutil.MaterialIcon("progress_activity", "spinner-icon"))
		rescan()
	})
	scanBtnWrapper := gtk.NewBox(gtk.OrientationHorizontal, 0)
	scanBtnWrapper.SetHAlign(gtk.AlignEnd)
	scanBtnWrapper.Append(scanBtn)
	box.Append(scanBtnWrapper)

	if refs.Bluetooth != nil {
		go rescan()
	}

	// Subscribe to device list updates.
	b.Subscribe(bus.TopicBluetoothDevices, func(e bus.Event) {
		devices, ok := e.Data.([]state.BluetoothDevice)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			gtkutil.ClearChildren(&pairedListBox.Widget, pairedListBox.Remove)
			gtkutil.ClearChildren(&availableListBox.Widget, availableListBox.Remove)

			pairedCount := 0
			availableCount := 0

			for _, dev := range devices {
				if dev.Paired {
					pairedCount++
					row := newBTDeviceRow(parent, refs, dev, rescan)
					pairedListBox.Append(row)
				} else {
					availableCount++
					row := newBTDeviceRow(parent, refs, dev, rescan)
					availableListBox.Append(row)
				}
			}

			gtkutil.UpdateSectionHeader(pairedHeader, pairedCount)
			gtkutil.UpdateSectionHeader(availableHeader, availableCount)
			restoreScanBtn()
		})
	})

	return box
}

func newBTDeviceRow(parent *gtk.ApplicationWindow, refs *servicerefs.ServiceRefs, dev state.BluetoothDevice, rescan func()) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.AddCSSClass("conn-row")
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
			gtkutil.ClearChildren(&meta.Widget, meta.Remove)
			meta.Append(gtkutil.MaterialIcon("progress_activity", "spinner-icon"))
		}

		click := gtk.NewGestureClick()
		click.SetButton(1)
		click.ConnectPressed(func(_ int, _ float64, _ float64) {
			click.SetState(gtk.EventSequenceClaimed)
		})
		click.ConnectReleased(func(_ int, _ float64, _ float64) {
			switch {
			case dev.Connected:
				gtkutil.ConfirmDialog(
					parent,
					"Disconnect device",
					name,
					"Disconnect",
					func() {
						setLoading()
						go func() { refs.Bluetooth.DisconnectDevice(addr); rescan() }()
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
						go func() { refs.Bluetooth.ConnectDevice(addr); rescan() }()
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
						go func() { refs.Bluetooth.PairDevice(addr); rescan() }()
					},
				)
			}
		})
		row.AddController(click)
	}

	return row
}
