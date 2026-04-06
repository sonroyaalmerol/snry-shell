package layershell

// barHeight is the current bar exclusive zone height, updated dynamically
// when the bar window is allocated. Defaults to 52 before the bar reports in.
// All access is on the GTK main thread.
var barHeight = 52
var barHeightCallbacks []func(int)

// BarHeight returns the current bar height as reported by the bar surface.
func BarHeight() int { return barHeight }

// SetBarHeight is called by the bar surface on every size allocation.
// It updates the stored height and notifies all registered callbacks.
func SetBarHeight(h int) {
	if h == barHeight {
		return
	}
	barHeight = h
	for _, fn := range barHeightCallbacks {
		fn(h)
	}
}

// OnBarHeightChanged registers fn to be called whenever the bar height changes.
// fn is called on the GTK main thread.
func OnBarHeightChanged(fn func(int)) {
	barHeightCallbacks = append(barHeightCallbacks, fn)
}
