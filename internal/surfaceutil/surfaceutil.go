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

func AddEscapeToClose(win *gtk.ApplicationWindow) {
	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.ConnectKeyPressed(func(keyval, keycode uint, _ gdk.ModifierType) bool {
		if keyval == 0xff1b {
			win.SetVisible(false)
			return true
		}
		return false
	})
	win.AddController(keyCtrl)
}

func AddEscapeToCloseWithCallback(win *gtk.ApplicationWindow, onClose func()) {
	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.ConnectKeyPressed(func(keyval, keycode uint, _ gdk.ModifierType) bool {
		if keyval == 0xff1b {
			onClose()
			win.SetVisible(false)
			return true
		}
		return false
	})
	win.AddController(keyCtrl)
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

func FormatTime(seconds float64) string {
	s := int(seconds)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// asWidget extracts the embedded *gtk.Widget from any gtk.Widgetter
// using reflection, since baseWidget() is unexported.
func asWidget(w gtk.Widgetter) *gtk.Widget {
	v := reflect.ValueOf(w)
	if v.Kind() == reflect.Ptr {
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
		widget := asWidget(current)
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
	widget := asWidget(w)
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

// PopupPanelConfig configures a standard popup panel surface.
type PopupPanelConfig struct {
	Name      string      // window name (e.g. "snry-controls")
	Namespace string      // layer-shell namespace
	Action    string      // toggle action (e.g. "toggle-controls")
	CloseOn   []string    // sibling actions that dismiss this panel
	Align     gtk.Align   // horizontal alignment of the panel (AlignStart or AlignEnd)
}

// NewPopupPanel creates a fullscreen overlay window with click-to-close scrim,
// escape-to-close, top margin, and toggle/close-on-sibling subscriptions.
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
	clickGesture.SetPropagationLimit(gtk.LimitNone)
	clickGesture.ConnectReleased(func(_ int, _ float64, _ float64) {
		win.SetVisible(false)
	})
	clickBg.AddController(clickGesture)

	root := gtk.NewBox(gtk.OrientationHorizontal, 0)
	root.AddCSSClass("popup-overlay")
	root.SetHAlign(cfg.Align)
	root.SetVAlign(gtk.AlignStart)
	clickBg.Append(root)

	win.SetChild(clickBg)
	win.SetVisible(false)

	layershell.SetMargin(win, layershell.EdgeTop, layershell.BarExclusiveZone+8)
	AddEscapeToClose(win)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		action, _ := e.Data.(string)
		if action == cfg.Action {
			glib.IdleAdd(func() { win.SetVisible(!win.Visible()) })
		}
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
func PositionUnderTrigger(root *gtk.Box, trigger gtk.Widgetter, panelWidth, panelMargin int) {
	triggerX := WidgetXRelativeToRoot(trigger)
	triggerW := WidgetWidth(trigger)
	popupW := panelWidth + panelMargin*2
	monW := MonitorWidth()

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
