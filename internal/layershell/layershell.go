// Package layershell provides Go wrappers for the gtk4-layer-shell C library
// via CGo. This avoids the gotk4-layer-shell module which targets gtk/v3 types
// and is incompatible with gotk4 v0.3.0 (gtk/v4).
package layershell

// #cgo pkg-config: gtk4-layer-shell-0
/*
#include <stdlib.h>
#include <stdlib.h>

// Forward-declare the C types we need.
typedef struct _GdkMonitor GdkMonitor;
typedef struct _GtkWindow GtkWindow;
typedef struct _GtkWidget GtkWidget;

extern void gtk_layer_init_for_window(GtkWindow *window);
extern void gtk_layer_set_layer(GtkWindow *window, int layer);
extern void gtk_layer_set_anchor(GtkWindow *window, int edge, int anchor);
extern void gtk_layer_set_margin(GtkWindow *window, int edge, int margin);
extern void gtk_layer_set_exclusive_zone(GtkWindow *window, int zone);
extern void gtk_layer_set_keyboard_mode(GtkWindow *window, int mode);
extern void gtk_layer_set_namespace(GtkWindow *window, const char *namespace);
extern void gtk_layer_set_monitor(GtkWindow *window, GdkMonitor *monitor);
*/
import "C"

import (
	"unsafe"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type Layer int

const (
	LayerBackground Layer = iota
	LayerBottom
	LayerTop
	LayerOverlay
)

type Edge int

const (
	EdgeLeft Edge = iota
	EdgeRight
	EdgeTop
	EdgeBottom
)

type KeyboardMode int

const (
	KeyboardModeNone KeyboardMode = iota
	KeyboardModeExclusive
	KeyboardModeOnDemand
)

func WindowPtr(w any) *C.GtkWindow {
	type nativeGetter interface {
		Native() uintptr
	}
	if n, ok := w.(nativeGetter); ok {
		return (*C.GtkWindow)(unsafe.Pointer(n.Native()))
	}
	return nil
}

func InitForWindow(w any) {
	C.gtk_layer_init_for_window(WindowPtr(w))
}

func SetLayer(w any, layer Layer) {
	C.gtk_layer_set_layer(WindowPtr(w), C.int(layer))
}

func SetAnchor(w any, edge Edge, anchor bool) {
	var a C.int
	if anchor {
		a = 1
	}
	C.gtk_layer_set_anchor(WindowPtr(w), C.int(edge), a)
}

func SetMargin(w any, edge Edge, margin int) {
	C.gtk_layer_set_margin(WindowPtr(w), C.int(edge), C.int(margin))
}

func SetExclusiveZone(w any, zone int) {
	C.gtk_layer_set_exclusive_zone(WindowPtr(w), C.int(zone))
}

func SetKeyboardMode(w any, mode KeyboardMode) {
	C.gtk_layer_set_keyboard_mode(WindowPtr(w), C.int(mode))
}

func SetNamespace(w any, namespace string) {
	cstr := C.CString(namespace)
	C.gtk_layer_set_namespace(WindowPtr(w), cstr)
	C.free(unsafe.Pointer(cstr))
}

func SetMonitor(w any, monitor *gdk.Monitor) {
	var monPtr *C.GdkMonitor
	if monitor != nil {
		monPtr = (*C.GdkMonitor)(unsafe.Pointer(glib.InternObject(monitor).Native()))
	}
	C.gtk_layer_set_monitor(WindowPtr(w), monPtr)
}

type WindowConfig struct {
	Name          string
	Layer         Layer
	Anchors       map[Edge]bool
	Margins       map[Edge]int
	KeyboardMode  KeyboardMode
	ExclusiveZone int
	Namespace     string
	Monitor       *gdk.Monitor
}

func NewWindow(app *gtk.Application, cfg WindowConfig) *gtk.ApplicationWindow {
	win := gtk.NewApplicationWindow(app)
	win.SetDecorated(false)
	win.SetName(cfg.Name)

	InitForWindow(win)
	SetLayer(win, cfg.Layer)

	if cfg.Anchors != nil {
		for edge, anchor := range cfg.Anchors {
			SetAnchor(win, edge, anchor)
		}
	}

	if cfg.Margins != nil {
		for edge, margin := range cfg.Margins {
			SetMargin(win, edge, margin)
		}
	}

	if cfg.KeyboardMode != 0 {
		SetKeyboardMode(win, cfg.KeyboardMode)
	}

	if cfg.ExclusiveZone != 0 {
		SetExclusiveZone(win, cfg.ExclusiveZone)
	}

	if cfg.Namespace != "" {
		SetNamespace(win, cfg.Namespace)
	}

	if cfg.Monitor != nil {
		SetMonitor(win, cfg.Monitor)
	}

	// Track input device: hide cursor on touch, restore on mouse.
	installTouchCursorTracker(win)

	return win
}

// installTouchCursorTracker uses capture-phase event controllers on the window
// to detect the input device source. When a touchscreen is active the cursor
// is hidden and a "touch-active" CSS class is added to suppress :hover rules
// (which GTK synthesizes from touch but never reliably clears). When a mouse
// is active, the cursor is restored and "touch-active" is removed.
func installTouchCursorTracker(win *gtk.ApplicationWindow) {
	noneCursor := gdk.NewCursorFromName("none", nil)
	defaultCursor := gdk.NewCursorFromName("default", nil)

	setTouchActive := func(active bool) {
		surf, ok := win.Surface().(*gdk.Surface)
		if !ok {
			return
		}
		if active {
			win.AddCSSClass("touch-active")
			surf.SetCursor(noneCursor)
		} else {
			win.RemoveCSSClass("touch-active")
			surf.SetCursor(defaultCursor)
		}
	}

	// Catch touch begin/end at capture phase so touch-active is set before GTK
	// synthesizes the pointer-enter (hover) event for child widgets.
	legacy := gtk.NewEventControllerLegacy()
	legacy.SetPropagationPhase(gtk.PhaseCapture)
	legacy.ConnectEvent(func(ev gdk.Eventer) bool {
		switch gdk.BaseEvent(ev).EventType() {
		case gdk.TouchBegin, gdk.TouchUpdate:
			setTouchActive(true)
		}
		return false // never consume
	})
	win.AddController(legacy)

	// Detect real mouse motion to switch back out of touch mode.
	motion := gtk.NewEventControllerMotion()
	motion.SetPropagationPhase(gtk.PhaseCapture)
	motion.ConnectMotion(func(x, y float64) {
		d, ok := motion.CurrentEventDevice().(*gdk.Device)
		if !ok {
			return
		}
		setTouchActive(d.Source() == gdk.SourceTouchscreen)
	})
	win.AddController(motion)
}

func FullscreenAnchors() map[Edge]bool {
	return map[Edge]bool{
		EdgeTop: true, EdgeBottom: true, EdgeLeft: true, EdgeRight: true,
	}
}

func TopEdgeAnchors() map[Edge]bool {
	return map[Edge]bool{EdgeTop: true, EdgeLeft: true, EdgeRight: true}
}

func BottomEdgeAnchors() map[Edge]bool {
	return map[Edge]bool{EdgeBottom: true, EdgeLeft: true, EdgeRight: true}
}
