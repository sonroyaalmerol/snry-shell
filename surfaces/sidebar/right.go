// Package sidebar provides the right-edge sidebar surface with notifications,
// media controls, calendar, quick toggles, and utility widgets.
package sidebar

import (
	"log"
	"os"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
)

// Right is the right-edge sidebar showing notifications, media, calendar, and controls.
type Right struct {
	win *gtk.ApplicationWindow
	bus *bus.Bus
}

// NewRight creates the right sidebar.
func NewRight(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs) *Right {
	// Fullscreen overlay — the sidebar panel sits on the right, clicks on the
	// empty area dismiss it.
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:         "snry-sidebar-right",
		Layer:        layershell.LayerOverlay,
		Anchors:      layershell.FullscreenAnchors(),
		KeyboardMode: layershell.KeyboardModeOnDemand,
		ExclusiveZone: -1,
		Namespace:    "snry-sidebar-right",
	})

	r := &Right{win: win, bus: b}
	r.build(refs)
	win.SetVisible(false)

	logger := log.New(os.Stderr, "[sidebar] ", log.Lmsgprefix|log.Ltime)

	// Click on the background (not on the sidebar panel) closes it.
	clickGesture := gtk.NewGestureClick()
	clickGesture.SetButton(1)
	clickGesture.ConnectReleased(func(_ int, _ float64, _ float64) {
		logger.Printf("background click — closing sidebar")
		r.close()
	})
	win.AddController(clickGesture)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-sidebar" {
			logger.Printf("toggle-sidebar received, currently visible=%v", r.win.Visible())
			r.toggle()
		}
	})

	return r
}

func (r *Right) build(refs *servicerefs.ServiceRefs) {
	// Root overlay: fullscreen box with sidebar content packed to the right.
	root := gtk.NewBox(gtk.OrientationHorizontal, 0)
	root.AddCSSClass("sidebar-overlay")
	root.SetHAlign(gtk.AlignEnd)
	root.SetVAlign(gtk.AlignFill)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("sidebar-scroll")
	scroll.SetVExpand(true)

	panel := gtk.NewBox(gtk.OrientationVertical, 0)
	panel.AddCSSClass("sidebar-right")
	panel.SetMarginTop(12)
	panel.SetMarginBottom(12)
	panel.SetMarginStart(12)
	panel.SetMarginEnd(12)
	panel.SetSizeRequest(460, -1)

	// Top group: notifications
	panel.Append(newNotificationList(r.bus))

	// Center group: media controls + calendar
	panel.Append(buildMediaGroup(r.bus, refs.Mpris))
	panel.Append(buildCalendarGroup())

	// Quick toggles
	panel.Append(newQuickToggles(r.bus, refs))

	// Pomodoro timer
	panel.Append(newPomodoroWidget(r.bus, refs))

	// Todo list
	panel.Append(newTodoWidget(r.bus, refs))

	// Volume mixer
	panel.Append(newVolumeMixerWidget(r.bus, refs))

	// WiFi networks
	panel.Append(newWiFiWidget(r.bus, refs))

	// Bluetooth devices
	panel.Append(newBluetoothWidget(r.bus, refs))

	// Bottom group: system controls (collapsible)
	panel.Append(buildBottomGroup(r.bus, refs))

	scroll.SetChild(panel)
	root.Append(scroll)
	r.win.SetChild(root)
}

// toggle shows or hides the sidebar.
func (r *Right) toggle() {
	logger := log.New(os.Stderr, "[sidebar] ", log.Lmsgprefix|log.Ltime)
	logger.Printf("toggle: visible=%v", r.win.Visible())
	if r.win.Visible() {
		r.close()
	} else {
		r.win.SetVisible(true)
	}
}

// close hides the sidebar.
func (r *Right) close() {
	r.win.SetVisible(false)
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
