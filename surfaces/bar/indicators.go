package bar

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/puzpuzpuz/xsync/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

// barGroup wraps a widget in a rounded container matching illogical-impulse BarGroup.
func barGroup(child gtk.Widgetter, position string) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("bar-group")
	if position == "bottom" {
		box.AddCSSClass("bar-group-bottom")
	} else {
		box.AddCSSClass("bar-group-top")
	}
	box.SetVAlign(gtk.AlignCenter)
	box.Append(child)
	return box
}

// clickableBarGroup wraps a widget like barGroup but adds a click gesture
// that publishes the given action string to TopicSystemControls.
func clickableBarGroup(child gtk.Widgetter, b *bus.Bus, action string, mon *gdk.Monitor, position string) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("bar-group")
	box.AddCSSClass("bar-group-clickable")
	if position == "bottom" {
		box.AddCSSClass("bar-group-bottom")
	} else {
		box.AddCSSClass("bar-group-top")
	}
	box.SetVAlign(gtk.AlignCenter)
	box.SetCursorFromName("pointer")
	box.Append(child)

	click := gtk.NewGestureClick()
	click.SetButton(1)
	click.ConnectReleased(func(_ int, _ float64, _ float64) {
		b.Publish(bus.TopicPopupTrigger, surfaceutil.PopupTrigger{
			Action: action, Trigger: box, Monitor: mon,
		})
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

// newAppDrawerIcon returns the app drawer icon.
func newAppDrawerIcon() gtk.Widgetter {
	icon := gtkutil.MaterialIcon("apps")
	icon.AddCSSClass("indicator-icon")
	return icon
}

// newNotificationIcon returns a single notification icon with unread badge.
func newNotificationIcon(b *bus.Bus) gtk.Widgetter {
	var count xsync.Counter
	icon := gtkutil.MaterialIcon("notifications")
	icon.AddCSSClass("indicator-icon")

	b.Subscribe(bus.TopicNotification, func(e bus.Event) {
		if e.Data == nil {
			count.Dec()
		} else if _, ok := e.Data.(state.Notification); ok {
			count.Inc()
		}
		c := int(count.Value())
		glib.IdleAdd(func() {
			if c > 0 {
				icon.SetText("notifications_active")
			} else {
				icon.SetText("notifications")
			}
		})
	})
	return icon
}

// newNetworkIcon returns a single network status icon (wifi or ethernet).
func newNetworkIcon(b *bus.Bus) gtk.Widgetter {
	icon := gtkutil.MaterialIcon("wifi_off")
	icon.AddCSSClass("indicator-icon")

	b.Subscribe(bus.TopicNetwork, func(e bus.Event) {
		ns := e.Data.(state.NetworkState)
		glib.IdleAdd(func() {
			if !ns.Connected {
				icon.SetText("wifi_off")
				return
			}

			switch ns.Type {
			case "ethernet":
				icon.SetText("settings_ethernet")
			case "wifi":
				icon.SetText("wifi")
			default:
				icon.SetText("wifi")
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

// newOskIcon returns a keyboard icon for toggling the on-screen keyboard.
func newOskIcon(b *bus.Bus) gtk.Widgetter {
	icon := gtkutil.MaterialIcon("keyboard")
	icon.AddCSSClass("indicator-icon")
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

	showPct := true

	b.Subscribe(bus.TopicSettingsChanged, func(e bus.Event) {
		if cfg, ok := e.Data.(settings.Config); ok {
			glib.IdleAdd(func() {
				showPct = cfg.BarShowBatteryPct
				valueLabel.SetVisible(showPct)
			})
		}
	})

	b.Subscribe(bus.TopicBattery, func(e bus.Event) {
		bs := e.Data.(state.BatteryState)
		glib.IdleAdd(func() {
			if !bs.Present {
				revealer.SetRevealChild(false)
				return
			}
			icon.SetText(batteryIcon(bs.Percentage, bs.Charging))
			valueLabel.SetText(fmt.Sprintf("%d%%", int(bs.Percentage)))
			valueLabel.SetVisible(showPct)
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
