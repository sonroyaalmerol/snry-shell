package surfaceutil

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
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

// baseWidgetter accesses the unexported baseWidget method to reach
// the underlying *gtk.Widget for Allocation/Parent calls.
type baseWidgetter interface {
	baseWidget() *gtk.Widget
}

// asWidget safely extracts the *gtk.Widget from any gtk.Widgetter.
func asWidget(w gtk.Widgetter) *gtk.Widget {
	if bw, ok := w.(baseWidgetter); ok {
		return bw.baseWidget()
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
