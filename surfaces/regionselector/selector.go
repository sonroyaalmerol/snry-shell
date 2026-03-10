// Package regionselector provides a full-screen region screenshot tool.
package regionselector

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
)

type RegionSelector struct {
	win      *gtk.ApplicationWindow
	bus      *bus.Bus
	area     *gtk.DrawingArea
	x1, y1   float64
	dragging bool
}

func New(app *gtk.Application, b *bus.Bus) *RegionSelector {
	win := gtk.NewApplicationWindow(app)
	win.SetDecorated(false)
	win.SetName("snry-region-selector")

	layershell.InitForWindow(win)
	layershell.SetLayer(win, layershell.LayerOverlay)
	layershell.SetAnchor(win, layershell.EdgeTop, true)
	layershell.SetAnchor(win, layershell.EdgeBottom, true)
	layershell.SetAnchor(win, layershell.EdgeLeft, true)
	layershell.SetAnchor(win, layershell.EdgeRight, true)
	layershell.SetKeyboardMode(win, layershell.KeyboardModeNone)
	layershell.SetExclusiveZone(win, -1)
	layershell.SetNamespace(win, "snry-region-selector")

	rs := &RegionSelector{win: win, bus: b}

	area := gtk.NewDrawingArea()
	area.AddCSSClass("region-selector")
	area.SetHExpand(true)
	area.SetVExpand(true)

	area.SetDrawFunc(func(da *gtk.DrawingArea, cr *cairo.Context, w, h int) {
		cr.SetSourceRGBA(0, 0, 0, 0)
		cr.Paint()

		if rs.dragging {
			cr.SetSourceRGBA(0.2, 0.6, 1.0, 0.3)
			x, y := rs.x1, rs.y1
			dx, dy := float64(w)-x, float64(h)-y
			cr.Rectangle(x, y, dx, dy)
			cr.Fill()
		}

		// Draw crosshair cursor.
		cr.SetSourceRGBA(1, 1, 1, 0.8)
		cx, cy := float64(w)/2, float64(h)/2
		cr.SetLineWidth(1)
		cr.MoveTo(cx-10, cy)
		cr.LineTo(cx+10, cy)
		cr.Stroke()
		cr.MoveTo(cx, cy-10)
		cr.LineTo(cx, cy+10)
		cr.Stroke()
	})

	// Implement drag with gesture click + motion.
	gesture := gtk.NewGestureDrag()
	gesture.SetButton(1)
	gesture.ConnectDragBegin(func(x, y float64) {
		rs.x1, rs.y1 = x, y
		rs.dragging = true
	})
	gesture.ConnectDragUpdate(func(x, y float64) {
		rs.x1, rs.y1 = x, y
	})
	gesture.ConnectDragEnd(func(x, y float64) {
		if !rs.dragging {
			return
		}
		rs.dragging = false
		rs.capture(int(rs.x1), int(rs.y1), int(x), int(y))
		win.SetVisible(false)
	})

	area.AddController(gesture)

	// Also handle via EventControllerMotion for the crosshair.
	motion := gtk.NewEventControllerMotion()
	motion.ConnectMotion(func(x, y float64) {
		area.QueueDraw()
	})
	area.AddController(motion)

	win.SetChild(area)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-region-selector" {
			glib.IdleAdd(func() { win.SetVisible(!win.Visible()) })
		}
	})

	// Escape to cancel.
	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.ConnectKeyPressed(func(keyval, keycode uint, _ gdk.ModifierType) bool {
		if keyval == 0xff1b {
			win.SetVisible(false)
			return true
		}
		return false
	})
	win.AddController(keyCtrl)

	win.SetVisible(false)
	return rs
}

func (rs *RegionSelector) capture(x1, y1, x2, y2 int) {
	// Normalize coordinates.
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}

	geom := fmt.Sprintf("%d,%d %dx%d", x1, y1, x2-x1, y2-y1)
	path := fmt.Sprintf("%s/Pictures/Screenshots/%s.png",
		os.ExpandEnv("$HOME"),
		time.Now().Format("2006-01-02_15-04-05"))

	go func() {
		_ = exec.Command("sh", "-c", fmt.Sprintf("grim -g '%s' %s && wl-copy < %s", geom, path, path)).Run()
	}()
}
