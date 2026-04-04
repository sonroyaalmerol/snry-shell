// Package osd provides the on-screen display for volume and brightness feedback.
package osd

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const dismissDelay = 1500 * time.Millisecond

// OSD is a floating on-screen display for volume and brightness.
type OSD struct {
	win       *gtk.ApplicationWindow
	icon      *gtk.Label
	scale     *gtk.Scale
	timer     *time.Timer
	bus       *bus.Bus
	lastAudio state.AudioSink
}

// New creates the OSD window (hidden by default) and wires bus subscriptions.
func New(app *gtk.Application, b *bus.Bus) *OSD {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-osd",
		Layer:         layershell.LayerOverlay,
		Anchors:       map[layershell.Edge]bool{layershell.EdgeBottom: true},
		Margins:       map[layershell.Edge]int{layershell.EdgeBottom: 60},
		KeyboardMode:  layershell.KeyboardModeNone,
		ExclusiveZone: -1,
		Namespace:     "snry-osd",
	})

	o := &OSD{win: win, bus: b}
	o.build()

	b.Subscribe(bus.TopicAudio, func(e bus.Event) {
		sink := e.Data.(state.AudioSink)
		// Only show OSD when volume or mute state actually changes.
		if sink.Muted == o.lastAudio.Muted && sink.Volume == o.lastAudio.Volume {
			return
		}
		o.lastAudio = sink
		vol := sink.Volume
		if sink.Muted {
			vol = 0
		}
		icon := audioIcon(sink.Volume, sink.Muted)
		o.show(icon, vol)
	})

	b.Subscribe(bus.TopicBrightness, func(e bus.Event) {
		bs := e.Data.(state.BrightnessState)
		var pct float64
		if bs.Max > 0 {
			pct = float64(bs.Current) / float64(bs.Max)
		}
		o.show("brightness_medium", pct)
	})

	win.SetVisible(false)
	return o
}

func (o *OSD) build() {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("osd")
	box.SetVAlign(gtk.AlignCenter)
	box.SetHAlign(gtk.AlignCenter)

	o.icon = gtk.NewLabel("volume_up")
	o.icon.AddCSSClass("osd-icon")
	o.icon.SetVAlign(gtk.AlignCenter)

	o.scale = gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 1, 0.01)
	o.scale.AddCSSClass("osd-slider")
	o.scale.SetDrawValue(false)
	o.scale.SetSensitive(false) // display only
	o.scale.SetHExpand(true)
	o.scale.SetSizeRequest(200, -1)

	box.Append(o.icon)
	box.Append(o.scale)
	o.win.SetChild(box)
}

// show updates the OSD content and (re)starts the dismiss timer.
// Safe to call from any goroutine — marshals onto the GTK main thread.
func (o *OSD) show(iconName string, value float64) {
	glib.IdleAdd(func() {
		o.icon.SetText(iconName)
		o.scale.SetValue(value)
		o.win.SetVisible(true)

		if o.timer != nil {
			o.timer.Stop()
		}
		o.timer = time.AfterFunc(dismissDelay, func() {
			glib.IdleAdd(func() {
				o.win.SetVisible(false)
			})
		})
	})
}

func audioIcon(volume float64, muted bool) string {
	if muted {
		return "volume_off"
	}
	if volume < 0.33 {
		return "volume_down"
	}
	return "volume_up"
}
