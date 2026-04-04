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
*/
import "C"

import (
	"unsafe"

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
	type native interface {
		native() uintptr
	}
	if n, ok := w.(native); ok {
		p := n.native()
		return (*C.GtkWindow)(unsafe.Pointer(p))
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

func IsSupported() bool {
	return true
}

type WindowConfig struct {
	Name          string
	Layer         Layer
	Anchors       map[Edge]bool
	Margins       map[Edge]int
	KeyboardMode  KeyboardMode
	ExclusiveZone int
	Namespace     string
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

	return win
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

func RightEdgeAnchors() map[Edge]bool {
	return map[Edge]bool{EdgeTop: true, EdgeBottom: true, EdgeRight: true}
}
