package sidebar

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// newBluetoothWidget creates a Bluetooth device list widget.
func newBluetoothWidget(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.AddCSSClass("bt-widget")

	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	headerLabel := gtk.NewLabel("Bluetooth Devices")
	headerLabel.AddCSSClass("notif-group-header")
	headerLabel.SetHExpand(true)

	scanBtn := gtk.NewButton()
	scanBtn.AddCSSClass("bt-scan-btn")
	scanIcon := gtk.NewLabel("search")
	scanIcon.AddCSSClass("material-icon")
	scanBtn.SetChild(scanIcon)
	scanBtn.ConnectClicked(func() {
		if refs.Bluetooth != nil {
			go func() {
				_ = refs.Bluetooth.StartScan()
				_, _ = refs.Bluetooth.GetDevices()
			}()
		}
	})

	header.Append(headerLabel)
	header.Append(scanBtn)
	box.Append(header)

	listBox := gtk.NewListBox()
	listBox.AddCSSClass("bt-list")
	listBox.SetSelectionMode(gtk.SelectionNone)

	if refs.Bluetooth != nil {
		go func() {
			_ = refs.Bluetooth.StartScan()
			_, _ = refs.Bluetooth.GetDevices()
		}()
	}

	b.Subscribe(bus.TopicBluetoothDevices, func(e bus.Event) {
		devices, ok := e.Data.([]state.BluetoothDevice)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			child := listBox.FirstChild()
			for child != nil {
				next := child.(*gtk.Widget).NextSibling()
				listBox.Remove(child)
				child = next
			}

			for _, dev := range devices {
				row := newBTDeviceRow(refs, dev)
				listBox.Append(row)
			}
		})
	})

	box.Append(listBox)
	return box
}

func newBTDeviceRow(refs *servicerefs.ServiceRefs, dev state.BluetoothDevice) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 8)
	row.AddCSSClass("bt-device-row")

	if dev.Connected {
		row.AddCSSClass("bt-connected")
	}

	icon := gtk.NewLabel("bluetooth")
	icon.AddCSSClass("material-icon")
	icon.AddCSSClass("bt-device-icon")

	nameLabel := gtk.NewLabel(dev.Name)
	if nameLabel.Text() == "" {
		nameLabel.SetText(dev.Address)
	}
	nameLabel.AddCSSClass("bt-device-name")
	nameLabel.SetHExpand(true)
	nameLabel.SetHAlign(gtk.AlignStart)

	statusLabel := gtk.NewLabel("")
	if dev.Connected {
		statusLabel.SetText("Connected")
	} else if dev.Paired {
		statusLabel.SetText("Paired")
	}
	statusLabel.AddCSSClass("bt-device-status")

	row.Append(icon)
	row.Append(nameLabel)
	row.Append(statusLabel)

	if refs.Bluetooth != nil {
		actionBtn := gtk.NewButton()
		actionBtn.AddCSSClass("bt-action-btn")

		if dev.Connected {
			btnLabel := gtk.NewLabel("disconnect")
			btnLabel.AddCSSClass("material-icon")
			actionBtn.SetChild(btnLabel)
			addr := dev.Address
			actionBtn.ConnectClicked(func() {
				go refs.Bluetooth.DisconnectDevice(addr)
			})
		} else if dev.Paired {
			btnLabel := gtk.NewLabel("connect")
			btnLabel.AddCSSClass("material-icon")
			actionBtn.SetChild(btnLabel)
			addr := dev.Address
			actionBtn.ConnectClicked(func() {
				go refs.Bluetooth.ConnectDevice(addr)
			})
		} else {
			btnLabel := gtk.NewLabel("add")
			btnLabel.AddCSSClass("material-icon")
			actionBtn.SetChild(btnLabel)
			addr := dev.Address
			actionBtn.ConnectClicked(func() {
				go refs.Bluetooth.PairDevice(addr)
			})
		}

		row.Append(actionBtn)
	}

	return row
}
