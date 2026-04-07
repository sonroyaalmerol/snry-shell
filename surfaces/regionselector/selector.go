// Package regionselector provides a full-screen region screenshot tool.
package regionselector

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

type RegionSelector struct {
	win      *gtk.ApplicationWindow
	bus      *bus.Bus
	area     *gtk.DrawingArea
	x1, y1   float64
	dragging bool
}

func New(app *gtk.Application, b *bus.Bus) *RegionSelector {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-region-selector",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeNone,
		ExclusiveZone: -1,
		Namespace:     "snry-region-selector",
	})

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

	surfaceutil.AddToggleOn(b, win, "toggle-region-selector")

	// Escape to cancel.
	surfaceutil.AddEscapeToClose(win)

	win.SetVisible(false)
	return rs
}

func (rs *RegionSelector) capture(x1, y1, x2, y2 int) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}

	// Build geometry string and output path without any shell interpolation.
	geom := fmt.Sprintf("%d,%d %dx%d", x1, y1, x2-x1, y2-y1)
	dir := filepath.Join(os.Getenv("HOME"), "Pictures", "Screenshots")
	path := filepath.Join(dir, time.Now().Format("2006-01-02_15-04-05")+".png")

	go func() {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("screenshot: mkdir: %v", err)
			return
		}
		// Pass arguments directly — no shell, no injection risk.
		if err := exec.Command("grim", "-g", geom, path).Run(); err != nil {
			log.Printf("screenshot: grim: %v", err)
			return
		}
		f, err := os.Open(path)
		if err != nil {
			log.Printf("screenshot: open: %v", err)
			return
		}
		defer f.Close()
		cmd := exec.Command("wl-copy", "--type", "image/png")
		cmd.Stdin = f
		if err := cmd.Run(); err != nil {
			log.Printf("screenshot: wl-copy: %v", err)
		}
	}()
}
