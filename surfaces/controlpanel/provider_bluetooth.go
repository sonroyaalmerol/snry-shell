package controlpanel

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const (
	bluezService = "org.bluez"
	bluezAdapter = "/org/bluez/hci0"
	bluezAdapterIface = "org.bluez.Adapter1"
	bluezDeviceIface  = "org.bluez.Device1"
)

// btConfigProvider implements ConfigProvider for Bluetooth settings.
type btConfigProvider struct {
	conn       *dbus.Conn
	deviceList *gtk.Box
	powerSwitch *gtkutil.M3CustomSwitch
	settingPower bool
}

func newBTProviderWithConnection() ConfigProvider {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Printf("[CONTROLPANEL] bluetooth: cannot connect to system bus: %v", err)
		return nil
	}
	return &btConfigProvider{conn: conn}
}

func (b *btConfigProvider) Name() string  { return "Bluetooth" }
func (b *btConfigProvider) Icon() string  { return "bluetooth" }
func (b *btConfigProvider) Load() error   { return nil }
func (b *btConfigProvider) Save() error   { return nil }

func (b *btConfigProvider) Close() {
	if b.conn != nil {
		b.conn.Close()
	}
}

func (b *btConfigProvider) BuildWidget() gtk.Widgetter {
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

	title := gtk.NewLabel("Bluetooth Settings")
	title.AddCSSClass("settings-title")
	title.SetHAlign(gtk.AlignStart)
	box.Append(title)

	// Power toggle
	b.powerSwitch = gtkutil.M3Switch()
	b.powerSwitch.ConnectStateSet(func(on bool) bool {
		if b.settingPower {
			return false
		}
		go b.setPowered(on)
		return true
	})
	box.Append(gtkutil.SwitchRow("Bluetooth", b.powerSwitch))

	// Scan button
	scanBtn := gtkutil.M3IconButton("refresh", "conn-scan-btn")
	scanBtn.ConnectClicked(func() {
		scanBtn.SetSensitive(false)
		scanBtn.SetChild(gtkutil.M3Spinner())
		go func() {
			b.startScan()
			glib.IdleAdd(func() {
				scanBtn.SetSensitive(true)
				scanBtn.SetChild(gtkutil.MaterialIcon("refresh"))
				b.refreshDevices()
			})
		}()
	})
	scanBtnWrapper := gtk.NewBox(gtk.OrientationHorizontal, 0)
	scanBtnWrapper.SetHAlign(gtk.AlignEnd)
	scanBtnWrapper.Append(scanBtn)
	box.Append(scanBtnWrapper)

	// Device list
	b.deviceList = gtk.NewBox(gtk.OrientationVertical, 0)
	b.deviceList.AddCSSClass("conn-list")
	box.Append(b.deviceList)

	// Initial load
	b.refreshPower()
	go b.refreshDevices()

	// Monitor D-Bus signals for real-time updates
	go b.monitorSignals()

	scroll.SetChild(box)
	return scroll
}

func (b *btConfigProvider) refreshPower() {
	obj := b.conn.Object(bluezService, bluezAdapter)
	poweredV, err := obj.GetProperty(bluezAdapterIface + ".Powered")
	if err != nil {
		return
	}
	powered, _ := poweredV.Value().(bool)
	glib.IdleAdd(func() {
		b.settingPower = true
		b.powerSwitch.SetActive(powered)
		b.settingPower = false
	})
}

func (b *btConfigProvider) setPowered(on bool) {
	obj := b.conn.Object(bluezService, bluezAdapter)
	err := obj.SetProperty(bluezAdapterIface+".Powered", dbus.MakeVariant(on))
	if err != nil && !on {
		_ = obj.Call(bluezAdapterIface+".StopDiscovery", 0).Err
		_ = obj.SetProperty(bluezAdapterIface+".Powered", dbus.MakeVariant(false))
	}
	glib.IdleAdd(b.refreshPower)
}

func (b *btConfigProvider) startScan() {
	obj := b.conn.Object(bluezService, bluezAdapter)
	if err := obj.Call(bluezAdapterIface+".StartDiscovery", 0).Err; err != nil {
		_ = obj.Call(bluezAdapterIface+".StopDiscovery", 0).Err
		_ = obj.Call(bluezAdapterIface+".StartDiscovery", 0).Err
	}
}

func (b *btConfigProvider) getDevices() ([]state.BluetoothDevice, error) {
	managed := b.conn.Object(bluezService, "/")
	var result map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := managed.Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&result); err != nil {
		return nil, err
	}

	var devices []state.BluetoothDevice
	for path, ifaces := range result {
		if _, ok := ifaces[bluezDeviceIface]; !ok {
			continue
		}
		devObj := b.conn.Object(bluezService, path)

		name := ""
		if v, err := devObj.GetProperty(bluezDeviceIface + ".Name"); err == nil {
			name, _ = v.Value().(string)
		}
		paired := false
		if v, err := devObj.GetProperty(bluezDeviceIface + ".Paired"); err == nil {
			paired, _ = v.Value().(bool)
		}
		connected := false
		if v, err := devObj.GetProperty(bluezDeviceIface + ".Connected"); err == nil {
			connected, _ = v.Value().(bool)
		}
		icon := "bluetooth"
		if v, err := devObj.GetProperty(bluezDeviceIface + ".Icon"); err == nil {
			icon, _ = v.Value().(string)
		}
		trusted := false
		if v, err := devObj.GetProperty(bluezDeviceIface + ".Trusted"); err == nil {
			trusted, _ = v.Value().(bool)
		}

		if name == "" {
			name = fmt.Sprintf("Device %s", path)
		}

		devices = append(devices, state.BluetoothDevice{
			Address:   string(path),
			Name:      name,
			Paired:    paired,
			Connected: connected,
			Icon:      icon,
			Trusted:   trusted,
		})
	}

	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Connected != devices[j].Connected {
			return devices[i].Connected
		}
		if devices[i].Paired != devices[j].Paired {
			return devices[i].Paired
		}
		return devices[i].Name < devices[j].Name
	})

	return devices, nil
}

func (b *btConfigProvider) refreshDevices() {
	devices, err := b.getDevices()
	if err != nil {
		return
	}
	glib.IdleAdd(func() {
		// Clear existing
		for child := b.deviceList.FirstChild(); child != nil; child = b.deviceList.FirstChild() {
			b.deviceList.Remove(child)
		}

		if len(devices) == 0 {
			empty := gtk.NewLabel("No devices found")
			empty.AddCSSClass("conn-row-label")
			empty.SetHAlign(gtk.AlignCenter)
			empty.SetMarginTop(12)
			b.deviceList.Append(empty)
			return
		}

		for _, dev := range devices {
			b.deviceList.Append(b.buildDeviceRow(dev))
		}
	})
}

func (b *btConfigProvider) buildDeviceRow(dev state.BluetoothDevice) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.AddCSSClass("conn-row")
	if dev.Connected {
		row.AddCSSClass("conn-row-connected")
	}

	iconName := mapDeviceIcon(dev.Icon)
	icon := gtkutil.MaterialIcon(iconName)
	icon.AddCSSClass("conn-row-icon")
	row.Append(icon)

	info := gtk.NewBox(gtk.OrientationVertical, 2)
	nameLabel := gtk.NewLabel(dev.Name)
	nameLabel.AddCSSClass("conn-row-label")
	nameLabel.SetHAlign(gtk.AlignStart)
	info.Append(nameLabel)

	statusText := "Available"
	if dev.Connected {
		statusText = "Connected"
	} else if dev.Paired {
		statusText = "Paired"
	}
	statusLabel := gtk.NewLabel(statusText)
	statusLabel.AddCSSClass("conn-row-status")
	statusLabel.SetHAlign(gtk.AlignStart)
	info.Append(statusLabel)
	row.Append(info)

	// Action button
	btn := gtkutil.M3IconButton(actionIcon(dev), "settings-btn-small")
	btn.ConnectClicked(func() {
		go b.handleDeviceAction(dev)
	})
	btn.SetHAlign(gtk.AlignEnd)
	btn.SetHExpand(true)
	row.Append(btn)

	return row
}

func (b *btConfigProvider) handleDeviceAction(dev state.BluetoothDevice) {
	var err error
	switch {
	case dev.Connected:
		err = b.conn.Object(bluezService, dbus.ObjectPath(dev.Address)).Call(bluezDeviceIface+".Disconnect", 0).Err
	case dev.Paired:
		err = b.conn.Object(bluezService, dbus.ObjectPath(dev.Address)).Call(bluezDeviceIface+".Connect", 0).Err
	default:
		obj := b.conn.Object(bluezService, dbus.ObjectPath(dev.Address))
		if err = obj.Call(bluezDeviceIface+".Pair", 0).Err; err == nil {
			_ = obj.SetProperty(bluezDeviceIface+".Trusted", dbus.MakeVariant(true))
		}
	}
	if err != nil {
		log.Printf("[CONTROLPANEL] bluetooth action: %v", err)
	}
	b.refreshDevices()
}

func (b *btConfigProvider) monitorSignals() {
	ch := make(chan *dbus.Signal, 8)
	b.conn.Signal(ch)
	defer b.conn.RemoveSignal(ch)
	_ = b.conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	)

	for sig := range ch {
		// Filter to BlueZ signals
		if sig.Path != bluezAdapter && !isDevicePath(sig.Path) {
			continue
		}
		// Drain queued signals before handling.
		drain:
		for {
			select {
			case <-ch:
			default:
				break drain
			}
		}
		b.refreshPower()
		b.refreshDevices()
	}
}

func isDevicePath(path dbus.ObjectPath) bool {
	return strings.HasPrefix(string(path), "/org/bluez/hci0/dev_")
}

func actionIcon(dev state.BluetoothDevice) string {
	switch {
	case dev.Connected:
		return "link_off"
	case dev.Paired:
		return "link"
	default:
		return "add"
	}
}

func mapDeviceIcon(icon string) string {
	switch icon {
	case "input-keyboard":
		return "keyboard"
	case "input-mouse":
		return "mouse"
	case "input-gaming":
		return "sports_esports"
	case "audio-headset":
		return "headset"
	case "audio-headphones":
		return "headphones"
	case "audio-speakers":
		return "speaker"
	case "phone":
		return "phone_android"
	default:
		return "bluetooth"
	}
}
