package widgets

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// buildSliderRow creates a labeled slider row with an icon.
func BuildSliderRow(icon, label string, min, max, step float64, onChange func(float64)) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 8)
	row.AddCSSClass("control-slider-row")

	iconLabel := gtk.NewLabel(icon)
	iconLabel.AddCSSClass("material-icon")
	iconLabel.AddCSSClass("control-icon")

	nameLabel := gtk.NewLabel(label)
	nameLabel.AddCSSClass("control-label")
	nameLabel.SetHAlign(gtk.AlignStart)

	scale := gtk.NewScaleWithRange(gtk.OrientationHorizontal, min, max, step)
	scale.AddCSSClass("control-scale")
	scale.SetDrawValue(false)
	scale.SetHExpand(true)
	scale.ConnectChangeValue(func(_ gtk.ScrollType, value float64) bool {
		onChange(value)
		return false
	})

	row.Append(iconLabel)
	row.Append(nameLabel)
	row.Append(scale)
	return row
}

// BuildQuickControls creates a box containing volume and brightness sliders.
func BuildQuickControls(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	controls := gtk.NewBox(gtk.OrientationVertical, 0)
	controls.AddCSSClass("system-controls")

	// Volume slider.
	controls.Append(BuildSliderRow("volume_up", "Volume", 0, 1, 0.01, func(val float64) {
		refs.Audio.SetVolume(val)
	}))

	// Brightness slider with live state sync.
	brightnessScale := gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 1, 0.01)
	brightnessScale.AddCSSClass("control-scale")
	brightnessScale.SetDrawValue(false)
	brightnessScale.SetHExpand(true)

	settingBrightness := false
	brightnessScale.ConnectChangeValue(func(_ gtk.ScrollType, value float64) bool {
		if !settingBrightness {
			refs.Brightness.SetBrightness(value)
		}
		return false
	})

	b.Subscribe(bus.TopicBrightness, func(e bus.Event) {
		bs, ok := e.Data.(state.BrightnessState)
		if !ok || bs.Max == 0 {
			return
		}
		glib.IdleAdd(func() {
			settingBrightness = true
			brightnessScale.SetValue(float64(bs.Current) / float64(bs.Max))
			settingBrightness = false
		})
	})

	brightnessRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	brightnessRow.AddCSSClass("control-slider-row")

	brightnessIcon := gtk.NewLabel("brightness_high")
	brightnessIcon.AddCSSClass("material-icon")
	brightnessIcon.AddCSSClass("control-icon")

	brightnessLabel := gtk.NewLabel("Brightness")
	brightnessLabel.AddCSSClass("control-label")
	brightnessLabel.SetHAlign(gtk.AlignStart)

	brightnessRow.Append(brightnessIcon)
	brightnessRow.Append(brightnessLabel)
	brightnessRow.Append(brightnessScale)
	controls.Append(brightnessRow)

	return controls
}
