package surfaceutil

import (
	"fmt"
	"reflect"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
)

// AddEscapeToClose dismisses win when Escape is pressed.
func AddEscapeToClose(win *gtk.ApplicationWindow) {
	AddEscapeToCloseWithCallback(win, func() {})
}

// AddEscapeToCloseWithCallback calls onClose then dismisses win when Escape is pressed.
func AddEscapeToCloseWithCallback(win *gtk.ApplicationWindow, onClose func()) {
	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.ConnectKeyPressed(func(keyval, _ uint, _ gdk.ModifierType) bool {
		if keyval == 0xff1b { // GDK_KEY_Escape
			onClose()
			win.SetVisible(false)
			return true
		}
		return false
	})
	win.AddController(keyCtrl)
}

// NewFullscreenOverlay creates a layer-shell window that covers the entire screen
// in the overlay layer with ExclusiveZone=-1. Use KeyboardModeExclusive for
// surfaces that must capture all input (session menu, settings); KeyboardModeOnDemand
// for surfaces that share the keyboard with other windows.
func NewFullscreenOverlay(app *gtk.Application, name string, kbMode layershell.KeyboardMode) *gtk.ApplicationWindow {
	return layershell.NewWindow(app, layershell.WindowConfig{
		Name:          name,
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  kbMode,
		ExclusiveZone: -1,
		Namespace:     name,
	})
}

func AddToggleOn(b *bus.Bus, win *gtk.ApplicationWindow, action string) {
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == action {
			glib.IdleAdd(func() { win.SetVisible(!win.Visible()) })
		}
	})
}

func AddToggleOnWithFocus(b *bus.Bus, win *gtk.ApplicationWindow, action string) {
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == action {
			glib.IdleAdd(func() {
				win.SetVisible(!win.Visible())
				if win.Visible() {
					win.GrabFocus()
				}
			})
		}
	})
}

func AddToggleOnWithCallback(win *gtk.ApplicationWindow, b *bus.Bus, action string, onToggle func()) {
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == action {
			glib.IdleAdd(onToggle)
		}
	})
}

func FormatTime(seconds float64) string {
	s := int(seconds)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// AsWidget extracts the embedded *gtk.Widget from any gtk.Widgetter
// using reflection.
func AsWidget(w gtk.Widgetter) *gtk.Widget {
	v := reflect.ValueOf(w)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	f := v.FieldByName("Widget")
	if !f.IsValid() {
		return nil
	}
	if f.CanAddr() {
		return f.Addr().Interface().(*gtk.Widget)
	}
	return nil
}

// WidgetXRelativeToRoot walks up the widget hierarchy accumulating x
// offsets so the result is relative to the root window (i.e. monitor coordinates).
func WidgetXRelativeToRoot(w gtk.Widgetter) int {
	x := 0
	for current := w; current != nil; {
		widget := AsWidget(current)
		if widget == nil {
			break
		}
		alloc := widget.Allocation()
		x += int(alloc.X())
		parent := widget.Parent()
		if parent == nil {
			break
		}
		current = parent
	}
	return x
}

// WidgetWidth returns the allocation width of a widget.
func WidgetWidth(w gtk.Widgetter) int {
	widget := AsWidget(w)
	if widget == nil {
		return 0
	}
	return int(widget.Allocation().Width())
}

// MonitorWidth returns the width of the primary monitor.
func MonitorWidth() int {
	display := gdk.DisplayGetDefault()
	if display == nil {
		return 0
	}
	monitors := display.Monitors()
	if monitors.NItems() == 0 {
		return 0
	}
	item := monitors.Item(0)
	if item == nil {
		return 0
	}
	mon := &gdk.Monitor{Object: item}
	geom := mon.Geometry()
	return geom.Width()
}

// MonitorHeight returns the height of the given monitor, or primary monitor if nil.
func MonitorHeight(mon *gdk.Monitor) int {
	if mon != nil {
		geom := mon.Geometry()
		return geom.Height()
	}
	display := gdk.DisplayGetDefault()
	if display == nil {
		return 1080 // fallback
	}
	monitors := display.Monitors()
	if monitors.NItems() == 0 {
		return 1080 // fallback
	}
	item := monitors.Item(0)
	if item == nil {
		return 1080 // fallback
	}
	mon = &gdk.Monitor{Object: item}
	geom := mon.Geometry()
	return geom.Height()
}

// PopupMaxHeight calculates the maximum content height for a popup
// based on the monitor height. It reserves space for the bar (top),
// margins, and some padding. Returns height in pixels.
func PopupMaxHeight(mon *gdk.Monitor, barHeight int) int {
	screenH := MonitorHeight(mon)
	// Reserve bar height + margin at top, and some bottom margin
	available := screenH - barHeight - 48
	// Use 70% of available space for the popup content
	maxH := int(float64(available) * 0.70)
	// Clamp to reasonable min/max values
	if maxH < 300 {
		return 300
	}
	if maxH > 800 {
		return 800
	}
	return maxH
}

// PopupTrigger carries the widget and monitor for a popup trigger click.
// Published to TopicPopupTrigger before TopicSystemControls so popups
// can update their trigger reference before toggling.
type PopupTrigger struct {
	Action  string
	Trigger gtk.Widgetter
	Monitor *gdk.Monitor
}

// PopupPanelConfig configures a standard popup panel surface.
type PopupPanelConfig struct {
	Name      string    // window name (e.g. "snry-controls")
	Namespace string    // layer-shell namespace
	CloseOn   []string  // sibling actions that dismiss this panel
	Align     gtk.Align // horizontal alignment of the panel (AlignStart or AlignEnd)
}

// NewPopupPanel creates a fullscreen overlay window with click-to-close scrim,
// escape-to-close, top margin, and close-on-sibling subscriptions.
// The caller is responsible for subscribing to its own toggle action (including
// any positioning logic via PositionUnderTrigger).
// Returns (win, clickBg, root) where clickBg is the scrim for appending panel content
// and root is the positioned container.
func NewPopupPanel(app *gtk.Application, b *bus.Bus, cfg PopupPanelConfig) (*gtk.ApplicationWindow, *gtk.Box, *gtk.Box) {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          cfg.Name,
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeOnDemand,
		ExclusiveZone: -1,
		Namespace:     cfg.Namespace,
	})

	clickBg := gtk.NewBox(gtk.OrientationVertical, 0)
	clickBg.SetHExpand(true)
	clickBg.SetVExpand(true)
	clickGesture := gtk.NewGestureClick()
	clickGesture.SetButton(1)
	clickGesture.SetPropagationPhase(gtk.PhaseTarget)
	clickGesture.ConnectReleased(func(_ int, _ float64, _ float64) {
		win.SetVisible(false)
	})
	clickBg.AddController(clickGesture)
	// Block scroll events from reaching the window, which would shift
	// the entire overlay. ScrolledWindows inside the panel handle their
	// own scrolling via capture/target; unconsumed events bubble up
	// here and get swallowed.
	scrollCtrl := gtk.NewEventControllerScroll(gtk.EventControllerScrollVertical | gtk.EventControllerScrollHorizontal)
	scrollCtrl.SetPropagationPhase(gtk.PhaseBubble)
	scrollCtrl.ConnectScroll(func(_ float64, _ float64) bool { return true })
	clickBg.AddController(scrollCtrl)

	root := gtk.NewBox(gtk.OrientationHorizontal, 0)
	root.AddCSSClass("popup-overlay")
	root.SetHAlign(cfg.Align)
	root.SetVAlign(gtk.AlignStart)
	clickBg.Append(root)

	win.SetChild(clickBg)
	win.SetVisible(false)

	setTopMargin := func(barH int) {
		layershell.SetMargin(win, layershell.EdgeTop, barH+8)
	}
	setTopMargin(layershell.BarHeight())
	layershell.OnBarHeightChanged(setTopMargin)
	AddEscapeToClose(win)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		action, _ := e.Data.(string)
		for _, closeAction := range cfg.CloseOn {
			if action == closeAction && win.Visible() {
				glib.IdleAdd(func() { win.SetVisible(false) })
				return
			}
		}
	})

	return win, clickBg, root
}

// PositionUnderTrigger centers root under the trigger widget, clamping to monitor bounds.
// If mon is nil, uses the primary monitor width.
func PositionUnderTrigger(root *gtk.Box, trigger gtk.Widgetter, panelWidth, panelMargin int, mon *gdk.Monitor) {
	triggerX := WidgetXRelativeToRoot(trigger)
	triggerW := WidgetWidth(trigger)
	popupW := panelWidth + panelMargin*2
	monW := MonitorWidth()
	if mon != nil {
		geom := mon.Geometry()
		monW = geom.Width()
	}

	desiredLeft := triggerX + triggerW/2 - popupW/2
	if monW > 0 {
		if desiredLeft < panelMargin {
			desiredLeft = panelMargin
		}
		if desiredLeft+popupW > monW-panelMargin {
			desiredLeft = monW - panelMargin - popupW
		}
	}

	root.SetMarginStart(desiredLeft)
}
