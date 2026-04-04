package bar

import (
	"fmt"
	"sync/atomic"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
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

// barSeparator returns a thin vertical divider.
func barSeparator() gtk.Widgetter {
	sep := gtk.NewLabel("")
	sep.AddCSSClass("bar-separator")
	return sep
}

// newLeftSidebarButton creates the left sidebar toggle button.
func newLeftSidebarButton(b *bus.Bus) gtk.Widgetter {
	btn := gtk.NewButton()
	btn.AddCSSClass("bar-sidebar-btn")
	icon := materialIcon("menu")
	btn.SetChild(icon)
	btn.SetTooltipText("Toggle sidebar")
	btn.ConnectClicked(func() {
		b.Publish(bus.TopicSystemControls, "toggle-sidebar")
	})
	return btn
}

// newIndicatorPill creates the grouped indicator pill (notifications, wifi, bluetooth).
func newIndicatorPill(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("indicator-pill")
	box.SetVAlign(gtk.AlignCenter)

	// Notification count.
	var count atomic.Int32
	notiIcon := materialIcon("notifications")
	notiIcon.AddCSSClass("indicator-icon")
	box.Append(notiIcon)

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
				notiIcon.SetText(fmt.Sprintf("notifications_active (%d)", c))
			} else {
				notiIcon.SetText("notifications")
			}
		})
	})

	// Network icon.
	netIcon := materialIcon("wifi_off")
	netIcon.AddCSSClass("indicator-icon")
	box.Append(netIcon)

	b.Subscribe(bus.TopicNetwork, func(e bus.Event) {
		ns := e.Data.(state.NetworkState)
		glib.IdleAdd(func() {
			if ns.Connected {
				netIcon.SetText("wifi")
			} else {
				netIcon.SetText("wifi_off")
			}
		})
	})

	// Bluetooth icon.
	btIcon := materialIcon("bluetooth")
	btIcon.AddCSSClass("indicator-icon")
	btIcon.SetVisible(refs.Bluetooth != nil)
	box.Append(btIcon)

	return box
}

// newStatusWidgetGroup returns the grouped status indicators (resources, volume, brightness, battery, keyboard).
func newStatusWidgetGroup(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.SetVAlign(gtk.AlignCenter)

	box.Append(newResourceIndicator(b))
	box.Append(newVolumeIndicator(b))
	box.Append(newBrightnessIndicator(b))
	box.Append(newBatteryIndicator(b))
	box.Append(newKeyboardIndicator(b, refs.Hyprland))

	return box
}

// materialIcon creates a label rendering a Material Symbols ligature.
func materialIcon(name string) *gtk.Label {
	l := gtk.NewLabel(name)
	l.AddCSSClass("material-icon")
	return l
}

func newVolumeIndicator(b *bus.Bus) gtk.Widgetter {
	icon := materialIcon("volume_up")
	valueLabel := gtk.NewLabel("--")
	valueLabel.AddCSSClass("indicator-value")

	box := gtk.NewBox(gtk.OrientationHorizontal, 2)
	box.SetVAlign(gtk.AlignCenter)
	box.Append(icon)
	box.Append(valueLabel)

	b.Subscribe(bus.TopicAudio, func(e bus.Event) {
		sink := e.Data.(state.AudioSink)
		glib.IdleAdd(func() {
			switch {
			case sink.Muted:
				icon.SetText("volume_off")
			case sink.Volume < 0.33:
				icon.SetText("volume_down")
			default:
				icon.SetText("volume_up")
			}
			valueLabel.SetText(fmt.Sprintf("%d%%", int(sink.Volume*100)))
		})
	})
	return box
}

func newBrightnessIndicator(b *bus.Bus) gtk.Widgetter {
	icon := materialIcon("brightness_medium")
	valueLabel := gtk.NewLabel("--")
	valueLabel.AddCSSClass("indicator-value")

	box := gtk.NewBox(gtk.OrientationHorizontal, 2)
	box.SetVAlign(gtk.AlignCenter)
	box.Append(icon)
	box.Append(valueLabel)

	b.Subscribe(bus.TopicBrightness, func(e bus.Event) {
		bs := e.Data.(state.BrightnessState)
		glib.IdleAdd(func() {
			pct := 0
			if bs.Max > 0 {
				pct = bs.Current * 100 / bs.Max
			}
			valueLabel.SetText(fmt.Sprintf("%d%%", pct))
		})
	})
	return box
}

func newBatteryIndicator(b *bus.Bus) gtk.Widgetter {
	revealer := gtk.NewRevealer()
	revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideLeft)
	revealer.SetTransitionDuration(200)

	icon := materialIcon("battery_full")
	valueLabel := gtk.NewLabel("")
	valueLabel.AddCSSClass("indicator-value")

	box := gtk.NewBox(gtk.OrientationHorizontal, 2)
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
	icon := materialIcon("language")
	icon.AddCSSClass("indicator-icon")

	label := gtk.NewLabel("")
	label.AddCSSClass("indicator-value")

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
