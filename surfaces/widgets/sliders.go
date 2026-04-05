package widgets

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// BuildSliderRow creates a labeled slider row with an icon.
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
	scale.AddCSSClass("m3-scale")
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
