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
func NewBluetoothWidget(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("conn-widget")

	// Header row: label + switch.
	header := gtk.NewBox(gtk.OrientationHorizontal, 12)
	header.AddCSSClass("conn-header")

	icon := gtkutil.MaterialIcon("bluetooth")
	icon.AddCSSClass("conn-header-icon")

	label := gtk.NewLabel("Bluetooth")
	label.AddCSSClass("conn-header-label")
	label.SetHExpand(true)

	sw := gtk.NewSwitch()
	sw.AddCSSClass("conn-switch")

	header.Append(icon)
	header.Append(label)
	header.Append(sw)
	box.Append(header)

	scanAction := func() {
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

	availableHeader := gtkutil.SectionHeader("Available devices", 0, availableRevealer, scanAction)
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
	scanBtn := gtkutil.MaterialButtonWithClass("search", "conn-scan-btn")
	scanBtn.ConnectClicked(scanAction)
	scanBtnWrapper := gtk.NewBox(gtk.OrientationHorizontal, 0)
	scanBtnWrapper.SetHAlign(gtk.AlignEnd)
	scanBtnWrapper.Append(scanBtn)
	box.Append(scanBtnWrapper)

	// Switch toggles Bluetooth on/off.
	if refs.Bluetooth != nil {
		sw.ConnectStateSet(func(val bool) bool {
			go refs.Bluetooth.SetPowered(val)
			return true
		})

		b.Subscribe(bus.TopicBluetooth, func(e bus.Event) {
			bs, ok := e.Data.(state.BluetoothState)
			if !ok {
				return
			}
			glib.IdleAdd(func() {
				sw.SetActive(bs.Powered)
			})
		})

		go scanAction()
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
					row := newBTDeviceRow(refs, dev)
					pairedListBox.Append(row)
				} else {
					availableCount++
					row := newBTDeviceRow(refs, dev)
					availableListBox.Append(row)
				}
			}

			gtkutil.UpdateSectionHeader(pairedHeader, pairedCount)
			gtkutil.UpdateSectionHeader(availableHeader, availableCount)
		})
	})

	return box
}

func newBTDeviceRow(refs *servicerefs.ServiceRefs, dev state.BluetoothDevice) gtk.Widgetter {
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
		click := gtk.NewGestureClick()
		click.SetButton(1)
		click.ConnectPressed(func(_ int, _ float64, _ float64) {
			click.SetState(gtk.EventSequenceClaimed)
		})
		click.ConnectReleased(func(_ int, _ float64, _ float64) {
			switch {
			case dev.Connected:
				gtkutil.ConfirmDialog(
					"Disconnect device",
					name,
					"Disconnect",
					func() { go refs.Bluetooth.DisconnectDevice(dev.Address) },
				)
			case dev.Paired:
				gtkutil.ConfirmDialog(
					"Connect to device",
					name,
					"Connect",
					func() { go refs.Bluetooth.ConnectDevice(dev.Address) },
				)
			default:
				gtkutil.ConfirmDialog(
					"Pair with device",
					name,
					"Pair",
					func() { go refs.Bluetooth.PairDevice(dev.Address) },
				)
			}
		})
		row.AddController(click)
	}

	return row
}
