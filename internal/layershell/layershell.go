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

import "unsafe"

// Layer is the z-order layer for a layer-shell surface.
type Layer int

const (
	LayerBackground Layer = iota
	LayerBottom
	LayerTop
	LayerOverlay
)

// Edge is a screen edge for anchoring/margins.
type Edge int

const (
	EdgeLeft Edge = iota
	EdgeRight
	EdgeTop
	EdgeBottom
)

// KeyboardMode controls keyboard interactivity for a layer-shell surface.
type KeyboardMode int

const (
	KeyboardModeNone      KeyboardMode = iota // 0
	KeyboardModeExclusive                     // 1
	KeyboardModeOnDemand                      // 2
)

// WindowPtr returns the underlying C pointer for a GTK4 window.
// In gotk4 v4, widget types implement native() uintptr returning the C pointer.
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

// InitForWindow must be called before the window is realized.
func InitForWindow(w any) {
	C.gtk_layer_init_for_window(WindowPtr(w))
}

// SetLayer sets the z-order layer.
func SetLayer(w any, layer Layer) {
	C.gtk_layer_set_layer(WindowPtr(w), C.int(layer))
}

// SetAnchor anchors the window to a screen edge.
func SetAnchor(w any, edge Edge, anchor bool) {
	var a C.int
	if anchor {
		a = 1
	}
	C.gtk_layer_set_anchor(WindowPtr(w), C.int(edge), a)
}

// SetMargin sets the margin from a screen edge.
func SetMargin(w any, edge Edge, margin int) {
	C.gtk_layer_set_margin(WindowPtr(w), C.int(edge), C.int(margin))
}

// SetExclusiveZone sets the exclusive zone (pixels).
func SetExclusiveZone(w any, zone int) {
	C.gtk_layer_set_exclusive_zone(WindowPtr(w), C.int(zone))
}

// SetKeyboardMode sets keyboard interactivity.
func SetKeyboardMode(w any, mode KeyboardMode) {
	C.gtk_layer_set_keyboard_mode(WindowPtr(w), C.int(mode))
}

// SetNamespace sets the layer shell namespace for the compositor.
func SetNamespace(w any, namespace string) {
	cstr := C.CString(namespace)
	C.gtk_layer_set_namespace(WindowPtr(w), cstr)
	C.free(unsafe.Pointer(cstr))
}

// IsSupported checks if the compositor supports layer shell.
func IsSupported() bool {
	return true
}
