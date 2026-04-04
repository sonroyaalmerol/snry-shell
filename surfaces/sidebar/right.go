// Package sidebar provides the right-edge sidebar surface with notifications,
// media controls, calendar, quick toggles, and utility widgets.
package sidebar

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

// Right is the right-edge sidebar showing notifications, media, calendar, and controls.
type Right struct {
	win *gtk.ApplicationWindow
	bus *bus.Bus
}

// NewRight creates the right sidebar.
func NewRight(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs) *Right {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:         "snry-sidebar-right",
		Layer:        layershell.LayerOverlay,
		Anchors:      layershell.RightEdgeAnchors(),
		KeyboardMode: layershell.KeyboardModeOnDemand,
		Namespace:    "snry-sidebar-right",
	})

	r := &Right{win: win, bus: b}
	r.build(refs)
	win.SetVisible(false)

	surfaceutil.AddToggleOn(b, win, "toggle-sidebar")

	return r
}

func (r *Right) build(refs *servicerefs.ServiceRefs) {
	root := gtk.NewBox(gtk.OrientationVertical, 0)
	root.AddCSSClass("sidebar-right")
	root.SetMarginTop(12)
	root.SetMarginBottom(12)
	root.SetMarginStart(12)
	root.SetMarginEnd(12)

	// Top group: notifications
	root.Append(newNotificationList(r.bus))

	// Center group: media controls + calendar
	root.Append(buildMediaGroup(r.bus, refs.Mpris))
	root.Append(buildCalendarGroup())

	// Quick toggles
	root.Append(newQuickToggles(r.bus, refs))

	// Pomodoro timer
	root.Append(newPomodoroWidget(r.bus, refs))

	// Todo list
	root.Append(newTodoWidget(r.bus, refs))

	// Volume mixer
	root.Append(newVolumeMixerWidget(r.bus, refs))

	// WiFi networks
	root.Append(newWiFiWidget(r.bus, refs))

	// Bluetooth devices
	root.Append(newBluetoothWidget(r.bus, refs))

	// Bottom group: system controls (collapsible)
	root.Append(buildBottomGroup(r.bus, refs))

	r.win.SetChild(root)
}

// Toggle shows or hides the sidebar.
func (r *Right) Toggle() {
	r.win.SetVisible(!r.win.Visible())
}

// buildBottomGroup creates the collapsible system controls section.
func buildBottomGroup(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	outer := gtk.NewBox(gtk.OrientationVertical, 0)

	// Chevron toggle button.
	chevron := gtk.NewButton()
	chevron.AddCSSClass("bottom-group-chevron")
	chevronLabel := gtk.NewLabel("expand_more")
	chevronLabel.AddCSSClass("material-icon")
	chevron.SetChild(chevronLabel)
	chevron.SetHAlign(gtk.AlignEnd)

	collapsed := false
	revealer := gtk.NewRevealer()
	revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	revealer.SetTransitionDuration(250)
	revealer.SetRevealChild(true)

	chevron.ConnectClicked(func() {
		collapsed = !collapsed
		revealer.SetRevealChild(!collapsed)
		if collapsed {
			chevronLabel.SetText("expand_less")
		} else {
			chevronLabel.SetText("expand_more")
		}
	})
	header := gtk.NewBox(gtk.OrientationHorizontal, 0)
	header.SetHAlign(gtk.AlignFill)
	label := gtk.NewLabel("Controls")
	label.AddCSSClass("notif-group-header")
	label.SetHExpand(true)
	header.Append(label)
	header.Append(chevron)
	outer.Append(header)

	// Controls content.
	controls := gtk.NewBox(gtk.OrientationVertical, 0)
	controls.AddCSSClass("system-controls")
	revealer.SetChild(controls)

	// Volume slider.
	controls.Append(buildSliderRow("volume_up", "Volume", 0, 1, 0.01, func(val float64) {
		refs.Audio.SetVolume(val)
	}))

	// Brightness slider.
	controls.Append(buildSliderRow("brightness_high", "Brightness", 0, 1, 0.01, func(val float64) {
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

	outer.Append(revealer)
	return outer
}

// buildSliderRow creates a labeled slider row with an icon.
func buildSliderRow(icon, label string, min, max, step float64, onChange func(float64)) gtk.Widgetter {
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
