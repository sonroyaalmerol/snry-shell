// Package recorder provides a screen recording overlay using wf-recorder.
package recorder

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

// Overlay is a small floating pill for screen recording controls.
type Overlay struct {
	win       *gtk.ApplicationWindow
	recordBtn *gtk.Button
	timerLbl  *gtk.Label
	recording bool
	cmd       *exec.Cmd
	start     time.Time
	bus       *bus.Bus
}

func New(app *gtk.Application, b *bus.Bus) *Overlay {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-recorder",
		Layer:         layershell.LayerOverlay,
		Anchors:       map[layershell.Edge]bool{layershell.EdgeBottom: true},
		Margins:       map[layershell.Edge]int{layershell.EdgeBottom: 120},
		KeyboardMode:  layershell.KeyboardModeNone,
		ExclusiveZone: -1,
		Namespace:     "snry-recorder",
	})

	box := gtk.NewBox(gtk.OrientationHorizontal, 12)
	box.AddCSSClass("recorder-pill")
	box.SetHAlign(gtk.AlignCenter)

	o := &Overlay{win: win, bus: b}

	// Indicator dot.
	dot := gtk.NewLabel("●")
	dot.AddCSSClass("recorder-dot")

	// Record/stop button.
	o.recordBtn = gtk.NewButton()
	o.recordBtn.AddCSSClass("recorder-btn")
	recIcon := gtkutil.MaterialIcon("fiber_manual_record", "recorder-btn-icon")
	o.recordBtn.SetChild(recIcon)
	o.recordBtn.ConnectClicked(func() {
		o.toggle()
	})

	// Timer label.
	o.timerLbl = gtk.NewLabel("00:00")
	o.timerLbl.AddCSSClass("recorder-timer")

	box.Append(dot)
	box.Append(o.recordBtn)
	box.Append(o.timerLbl)
	win.SetChild(box)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-recorder" {
			glib.IdleAdd(func() {
				if win.Visible() {
					win.SetVisible(false)
				} else {
					win.SetVisible(true)
				}
			})
		}
	})

	surfaceutil.AddEscapeToCloseWithCallback(win, func() {
		if o.recording {
			o.stop()
		}
	})
	win.SetVisible(false)
	return o
}

func (o *Overlay) toggle() {
	if o.recording {
		o.stop()
	} else {
		o.startRecording()
	}
}

func (o *Overlay) startRecording() {
	home, _ := os.UserHomeDir()
	filename := fmt.Sprintf("%s/Videos/recording-%s.mp4", home, time.Now().Format("2006-01-02_15-04-05"))

	o.cmd = exec.Command("wf-recorder", "-f", filename)
	if err := o.cmd.Start(); err != nil {
		o.timerLbl.SetText("ERR")
		return
	}

	o.recording = true
	o.start = time.Now()
	o.recordBtn.AddCSSClass("recorder-btn-active")

	// Handle process exit.
	go func() {
		_ = o.cmd.Wait()
		glib.IdleAdd(func() {
			o.recording = false
			o.recordBtn.RemoveCSSClass("recorder-btn-active")
			o.timerLbl.SetText("00:00")
		})
	}()

	// Update timer every second.
	glib.IdleAdd(func() {
		o.updateTimer()
	})

	// Forward SIGINT to child.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	go func() {
		<-sigCh
		// Already handled by Wait above
	}()
}

func (o *Overlay) stop() {
	if o.cmd != nil && o.cmd.Process != nil {
		o.cmd.Process.Signal(syscall.SIGINT)
	}
}

func (o *Overlay) updateTimer() {
	if !o.recording || !o.win.Visible() {
		return
	}
	elapsed := time.Since(o.start)
	mins := int(elapsed.Minutes()) % 60
	secs := int(elapsed.Seconds()) % 60
	o.timerLbl.SetText(fmt.Sprintf("%02d:%02d", mins, secs))
	glib.TimeoutAdd(1000, func() bool {
		o.updateTimer()
		return false
	})
}
