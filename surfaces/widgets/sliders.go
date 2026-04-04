package widgets

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
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

// BuildQuickControls creates a box containing volume/brightness sliders and a wallpaper button.
func BuildQuickControls(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	controls := gtk.NewBox(gtk.OrientationVertical, 0)
	controls.AddCSSClass("system-controls")

	// Volume slider.
	controls.Append(BuildSliderRow("volume_up", "Volume", 0, 1, 0.01, func(val float64) {
		refs.Audio.SetVolume(val)
	}))

	// Brightness slider.
	controls.Append(BuildSliderRow("brightness_high", "Brightness", 0, 1, 0.01, func(val float64) {
		refs.Brightness.SetBrightness(val)
	}))

	// Wallpaper button.
	wpBtn := gtk.NewButton()
	wpLabel := gtk.NewLabel("wallpaper")
	wpLabel.AddCSSClass("toggle-label")
	wpBtn.SetChild(wpLabel)
	wpBtn.ConnectClicked(func() {
		b.Publish(bus.TopicSystemControls, "open-wallpaper-picker")
	})
	controls.Append(wpBtn)

	return controls
}
