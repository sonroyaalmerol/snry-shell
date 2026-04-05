package bar

import (
	"fmt"
	"sync/atomic"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// barGroup wraps a widget in a rounded container matching illogical-impulse BarGroup.
func barGroup(child gtk.Widgetter) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("bar-group")
	box.SetVAlign(gtk.AlignCenter)
	box.Append(child)
	return box
}

// clickableBarGroup wraps a widget like barGroup but adds a click gesture
// that publishes the given action string to TopicSystemControls.
func clickableBarGroup(child gtk.Widgetter, b *bus.Bus, action string) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("bar-group")
	box.AddCSSClass("bar-group-clickable")
	box.SetVAlign(gtk.AlignCenter)
	box.SetCursorFromName("pointer")
	box.Append(child)

	click := gtk.NewGestureClick()
	click.SetButton(1)
	click.ConnectReleased(func(_ int, _ float64, _ float64) {
		b.Publish(bus.TopicSystemControls, action)
	})
	box.AddController(click)

	return box
}

// barSeparator returns a thin vertical divider.
func barSeparator() gtk.Widgetter {
	sep := gtk.NewLabel("")
	sep.AddCSSClass("bar-separator")
	return sep
}

// newNotificationIcon returns a single notification icon with unread badge.
func newNotificationIcon(b *bus.Bus) gtk.Widgetter {
	var count atomic.Int32
	icon := gtkutil.MaterialIcon("notifications")
	icon.AddCSSClass("indicator-icon")

	b.Subscribe(bus.TopicNotification, func(e bus.Event) {
		if e.Data == nil {
			count.Add(-1)
		} else if _, ok := e.Data.(state.Notification); ok {
			count.Add(1)
		}
		c := int(count.Load())
		glib.IdleAdd(func() {
			if c > 0 {
				if c > 99 {
					c = 99
				}
				icon.SetText(fmt.Sprintf("notifications_active (%d)", c))
			} else {
				icon.SetText("notifications")
			}
		})
	})
	return icon
}

// newWifiIcon returns a single wifi status icon.
func newWifiIcon(b *bus.Bus) gtk.Widgetter {
	icon := gtkutil.MaterialIcon("wifi_off")
	icon.AddCSSClass("indicator-icon")

	b.Subscribe(bus.TopicNetwork, func(e bus.Event) {
		ns := e.Data.(state.NetworkState)
		glib.IdleAdd(func() {
			if ns.Connected {
				icon.SetText("wifi")
			} else {
				icon.SetText("wifi_off")
			}
		})
	})
	return icon
}

// newBluetoothIcon returns a single bluetooth status icon.
func newBluetoothIcon(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	icon := gtkutil.MaterialIcon("bluetooth_disabled")
	icon.AddCSSClass("indicator-icon")
	icon.SetVisible(refs.Bluetooth != nil)

	b.Subscribe(bus.TopicBluetooth, func(e bus.Event) {
		bs, ok := e.Data.(state.BluetoothState)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			if bs.Powered {
				if bs.Connected {
					icon.SetText("bluetooth_connected")
				} else {
					icon.SetText("bluetooth")
				}
			} else {
				icon.SetText("bluetooth_disabled")
			}
		})
	})
	return icon
}

// newStatusIcon returns a single icon that opens the control panel.
func newStatusIcon(b *bus.Bus) gtk.Widgetter {
	icon := gtkutil.MaterialIcon("tune")
	icon.AddCSSClass("indicator-icon")
	icon.SetTooltipText("Controls")
	return icon
}

// newBatteryIndicator returns a battery icon + percentage, hidden if no battery.
func newBatteryIndicator(b *bus.Bus) gtk.Widgetter {
	revealer := gtk.NewRevealer()
	revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideLeft)
	revealer.SetTransitionDuration(200)

	icon := gtkutil.MaterialIcon("battery_full")
	valueLabel := gtk.NewLabel("")
	valueLabel.AddCSSClass("bar-battery-value")

	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.SetVAlign(gtk.AlignCenter)
	box.Append(icon)
	box.Append(valueLabel)
	revealer.SetChild(box)

	b.Subscribe(bus.TopicBattery, func(e bus.Event) {
		bs := e.Data.(state.BatteryState)
		glib.IdleAdd(func() {
			if !bs.Present {
				revealer.SetRevealChild(false)
				return
			}
			icon.SetText(batteryIcon(bs.Percentage, bs.Charging))
			valueLabel.SetText(fmt.Sprintf("%d%%", int(bs.Percentage)))
			revealer.SetRevealChild(true)
		})
	})
	return revealer
}

func batteryIcon(pct float64, charging bool) string {
	if charging {
		return "battery_charging_full"
	}
	switch {
	case pct >= 90:
		return "battery_full"
	case pct >= 20:
		return "battery_5_bar"
	default:
		return "battery_alert"
	}
}

func newKeyboardIndicator(b *bus.Bus, querier *hyprland.Querier) gtk.Widgetter {
	icon := gtkutil.MaterialIcon("language")
	icon.AddCSSClass("indicator-icon")

	label := gtk.NewLabel("")
	label.AddCSSClass("bar-kbd-layout")

	box := gtk.NewBox(gtk.OrientationHorizontal, 2)
	box.SetVAlign(gtk.AlignCenter)
	box.Append(icon)
	box.Append(label)

	b.Subscribe(bus.TopicKeyboard, func(e bus.Event) {
		layout := e.Data.(string)
		glib.IdleAdd(func() { label.SetText(layout) })
	})

	if querier != nil {
		if layout, err := querier.ActiveKeymap(); err == nil {
			label.SetText(layout)
		}

		clickGesture := gtk.NewGestureClick()
		clickGesture.SetButton(1)
		clickGesture.ConnectReleased(func(_ int, _ float64, _ float64) {
			_ = querier.SwitchXkbLayout()
		})
		box.AddController(clickGesture)
	}

	return box
}
