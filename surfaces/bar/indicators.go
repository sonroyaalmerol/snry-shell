package bar

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// newIndicatorsWidget returns the right-zone box: media | indicators | clock.
func newIndicatorsWidget(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.SetVAlign(gtk.AlignCenter)

	box.Append(newMediaWidget(b))
	box.Append(newResourceIndicator(b))
	box.Append(newStatusIndicator(newBrightnessIndicator(b)))
	box.Append(newStatusIndicator(newVolumeIndicator(b)))
	box.Append(newStatusIndicator(newNetworkIndicator(b)))
	box.Append(newBatteryIndicator(b)) // battery wraps its own revealer
	box.Append(newKeyboardIndicator(b, refs.Hyprland))
	box.Append(newClockWidget())

	return box
}

// newStatusIndicator wraps a widget in a box with the status-indicator CSS class.
func newStatusIndicator(inner gtk.Widgetter) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("status-indicator")
	box.SetVAlign(gtk.AlignCenter)
	box.Append(inner)
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
	valueLabel.SetCSSClasses([]string{"bar-date"})

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
	valueLabel.SetCSSClasses([]string{"bar-date"})

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

func newNetworkIndicator(b *bus.Bus) gtk.Widgetter {
	icon := materialIcon("wifi_off")

	b.Subscribe(bus.TopicNetwork, func(e bus.Event) {
		ns := e.Data.(state.NetworkState)
		glib.IdleAdd(func() {
			if ns.Connected {
				icon.SetText("wifi")
				icon.SetTooltipText(ns.SSID)
			} else {
				icon.SetText("wifi_off")
				icon.SetTooltipText("Disconnected")
			}
		})
	})
	return icon
}

func newBatteryIndicator(b *bus.Bus) gtk.Widgetter {
	revealer := gtk.NewRevealer()
	revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideLeft)
	revealer.SetTransitionDuration(200)

	icon := materialIcon("battery_full")
	valueLabel := gtk.NewLabel("")
	valueLabel.SetCSSClasses([]string{"bar-date"})

	box := gtk.NewBox(gtk.OrientationHorizontal, 2)
	box.AddCSSClass("status-indicator")
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
