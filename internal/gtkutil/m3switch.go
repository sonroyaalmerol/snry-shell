package gtkutil

import (
	"math"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// M3CustomSwitch is a custom Material Design 3 switch widget.
// It provides exact M3 dimensions: 52x32dp track with 24x24dp circular thumb.
type M3CustomSwitch struct {
	*gtk.Box

	active    bool
	disabled  bool
	animation float64 // 0.0 to 1.0 for thumb position
	target    float64 // target animation value

	trackWidget *gtk.DrawingArea

	onChange   func(bool)
	onStateSet func(bool) bool
}

// NewM3CustomSwitch creates a new Material 3 custom switch.
func NewM3CustomSwitch() *M3CustomSwitch {
	sw := &M3CustomSwitch{
		Box:       gtk.NewBox(gtk.OrientationHorizontal, 0),
		target:    0.0,
		animation: 0.0,
	}

	// Create drawing area for the switch track and thumb
	sw.trackWidget = gtk.NewDrawingArea()
	sw.trackWidget.SetContentWidth(52)
	sw.trackWidget.SetContentHeight(32)
	sw.trackWidget.SetHAlign(gtk.AlignCenter)
	sw.trackWidget.SetVAlign(gtk.AlignCenter)

	// Set up the draw function
	sw.trackWidget.SetDrawFunc(func(area *gtk.DrawingArea, cr *cairo.Context, width, height int) {
		sw.drawSwitch(cr, width, height)
	})

	sw.Append(sw.trackWidget)

	// Add CSS class for styling hooks
	sw.AddCSSClass("m3-custom-switch")
	sw.trackWidget.AddCSSClass("m3-custom-switch-track")

	// Set up click handling
	clickCtrl := gtk.NewGestureClick()
	clickCtrl.SetButton(1) // Left click
	clickCtrl.ConnectPressed(func(nPress int, x, y float64) {
		if !sw.disabled {
			sw.toggle()
		}
	})
	sw.AddController(clickCtrl)

	// Set cursor
	sw.SetCursorFromName("pointer")

	// Start animation loop
	sw.startAnimation()

	return sw
}

// toggle switches the state and notifies listeners.
func (sw *M3CustomSwitch) toggle() {
	// If there's an onStateSet callback, let it decide
	if sw.onStateSet != nil {
		result := sw.onStateSet(!sw.active)
		if !result {
			return // Callback rejected the change
		}
	}
	sw.SetActive(!sw.active)
}

// SetActive sets the switch state.
func (sw *M3CustomSwitch) SetActive(active bool) {
	if sw.active == active {
		return
	}
	sw.active = active
	if active {
		sw.target = 1.0
	} else {
		sw.target = 0.0
	}

	// Notify via callback
	if sw.onChange != nil {
		sw.onChange(active)
	}
}

// Active returns the current switch state.
func (sw *M3CustomSwitch) Active() bool {
	return sw.active
}

// SetSensitive enables/disables the switch.
func (sw *M3CustomSwitch) SetSensitive(sensitive bool) {
	sw.disabled = !sensitive
	sw.Box.SetSensitive(sensitive)
	if sensitive {
		sw.RemoveCSSClass("disabled")
	} else {
		sw.AddCSSClass("disabled")
	}
	sw.trackWidget.QueueDraw()
}

// ConnectStateSet connects a callback for state changes.
// Returns false from callback to prevent the state change.
func (sw *M3CustomSwitch) ConnectStateSet(callback func(bool) bool) {
	sw.onStateSet = callback
}

// Connect connects a signal handler.
func (sw *M3CustomSwitch) Connect(signal string, callback interface{}) glib.SignalHandle {
	return sw.Box.Connect(signal, callback)
}

// startAnimation starts the animation loop for smooth thumb transitions.
func (sw *M3CustomSwitch) startAnimation() {
	glib.TimeoutAdd(16, func() bool {
		if math.Abs(sw.animation-sw.target) < 0.001 {
			if sw.animation != sw.target {
				sw.animation = sw.target
				sw.trackWidget.QueueDraw()
			}
			return true // Keep running
		}

		// Smooth interpolation (eased)
		sw.animation += (sw.target - sw.animation) * 0.25
		sw.trackWidget.QueueDraw()
		return true
	})
}

// drawSwitch draws the switch track and thumb.
func (sw *M3CustomSwitch) drawSwitch(cr *cairo.Context, width, height int) {
	// Track dimensions
	trackWidth := float64(width)
	trackHeight := float64(height)
	trackRadius := trackHeight / 2

	// Thumb dimensions (24dp)
	thumbSize := 24.0
	thumbRadius := thumbSize / 2

	// Calculate thumb position based on animation
	// Unchecked: thumb at left edge + margin
	// Checked: thumb at right edge - thumb size - margin
	margin := 4.0
	startX := margin + thumbRadius
	endX := trackWidth - margin - thumbRadius
	thumbX := startX + (endX-startX)*sw.animation
	thumbY := trackHeight / 2

	// Default colors - Material 3 specs
	var trackR, trackG, trackB, trackA float64
	var thumbR, thumbG, thumbB, thumbA float64
	var outlineR, outlineG, outlineB, outlineA float64

	if sw.disabled {
		// Disabled state per Material 3:
		// Track: surfaceVariant at 38% opacity
		// Outline: onSurface at 12% opacity
		// Thumb: surface at 38% opacity
		trackR, trackG, trackB = 0.88, 0.88, 0.88 // Surface variant (light)
		trackA = 0.38
		outlineR, outlineG, outlineB = 0.1, 0.1, 0.1 // On surface
		outlineA = 0.12
		thumbR, thumbG, thumbB = 0.95, 0.95, 0.95 // Surface (light)
		thumbA = 0.38
	} else if sw.active {
		// Checked state: primary color track, on-primary thumb
		trackR, trackG, trackB = 0.4, 0.31, 0.64 // Material 3 primary purple-ish (#6750A4)
		trackA = 1.0
		thumbR, thumbG, thumbB, thumbA = 1.0, 1.0, 1.0, 1.0 // On-primary (white)
		outlineA = 0                                        // No outline in checked state
	} else {
		// Unchecked state: surface container track with outline, outline thumb
		trackR, trackG, trackB = 0.9, 0.9, 0.9 // Surface container
		trackA = 0.8
		outlineR, outlineG, outlineB = 0.5, 0.5, 0.5 // Outline
		outlineA = 0.5
		thumbR, thumbG, thumbB = 0.5, 0.5, 0.5 // Outline thumb
		thumbA = 1.0
	}

	// Draw track background
	cr.SetSourceRGBA(trackR, trackG, trackB, trackA)
	cr.NewSubPath()
	cr.Arc(trackRadius, trackRadius, trackRadius, math.Pi/2, 3*math.Pi/2)
	cr.Arc(trackWidth-trackRadius, trackRadius, trackRadius, -math.Pi/2, math.Pi/2)
	cr.ClosePath()
	cr.Fill()

	// Draw outline for unchecked or disabled states
	if outlineA > 0 {
		cr.SetSourceRGBA(outlineR, outlineG, outlineB, outlineA)
		cr.SetLineWidth(2)
		cr.NewSubPath()
		cr.Arc(trackRadius, trackRadius, trackRadius-1, math.Pi/2, 3*math.Pi/2)
		cr.Arc(trackWidth-trackRadius, trackRadius, trackRadius-1, -math.Pi/2, math.Pi/2)
		cr.ClosePath()
		cr.Stroke()
	}

	// Draw thumb shadow (subtle drop shadow)
	shadowAlpha := 0.2
	if sw.disabled {
		shadowAlpha *= 0.38
	}
	cr.SetSourceRGBA(0, 0, 0, shadowAlpha)
	cr.Arc(thumbX, thumbY+1, thumbRadius, 0, 2*math.Pi)
	cr.Fill()

	// Draw thumb
	cr.SetSourceRGBA(thumbR, thumbG, thumbB, thumbA)
	cr.Arc(thumbX, thumbY, thumbRadius, 0, 2*math.Pi)
	cr.Fill()
}
