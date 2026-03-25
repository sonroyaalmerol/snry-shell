package sidebar

import (
	"fmt"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// newPomodoroWidget creates a pomodoro timer widget.
func newPomodoroWidget(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.AddCSSClass("pomodoro-widget")

	header := gtk.NewLabel("Pomodoro")
	header.AddCSSClass("notif-group-header")
	header.SetHAlign(gtk.AlignStart)
	box.Append(header)

	// Timer display.
	timerBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
	timerBox.AddCSSClass("pomodoro-timer")
	timerBox.SetHAlign(gtk.AlignCenter)

	phaseLabel := gtk.NewLabel("IDLE")
	phaseLabel.AddCSSClass("pomodoro-phase")

	timeLabel := gtk.NewLabel("25:00")
	timeLabel.AddCSSClass("pomodoro-time")

	sessionLabel := gtk.NewLabel("#0")
	sessionLabel.AddCSSClass("pomodoro-sessions")

	timerBox.Append(phaseLabel)
	timerBox.Append(timeLabel)
	timerBox.Append(sessionLabel)
	box.Append(timerBox)

	// Controls.
	controls := gtk.NewBox(gtk.OrientationHorizontal, 4)
	controls.SetHAlign(gtk.AlignCenter)
	controls.AddCSSClass("pomodoro-controls")

	startBtn := gtk.NewButton()
	startBtn.AddCSSClass("pomodoro-btn")
	startIcon := gtk.NewLabel("play_arrow")
	startIcon.AddCSSClass("material-icon")
	startBtn.SetChild(startIcon)
	startBtn.ConnectClicked(func() {
		if refs.Pomodoro != nil {
			refs.Pomodoro.Start()
		}
	})

	pauseBtn := gtk.NewButton()
	pauseBtn.AddCSSClass("pomodoro-btn")
	pauseIcon := gtk.NewLabel("pause")
	pauseIcon.AddCSSClass("material-icon")
	pauseBtn.SetChild(pauseIcon)
	pauseBtn.ConnectClicked(func() {
		if refs.Pomodoro != nil {
			refs.Pomodoro.Pause()
		}
	})

	resetBtn := gtk.NewButton()
	resetBtn.AddCSSClass("pomodoro-btn")
	resetIcon := gtk.NewLabel("replay")
	resetIcon.AddCSSClass("material-icon")
	resetBtn.SetChild(resetIcon)
	resetBtn.ConnectClicked(func() {
		if refs.Pomodoro != nil {
			refs.Pomodoro.Reset()
		}
	})

	skipBtn := gtk.NewButton()
	skipBtn.AddCSSClass("pomodoro-btn")
	skipIcon := gtk.NewLabel("skip_next")
	skipIcon.AddCSSClass("material-icon")
	skipBtn.SetChild(skipIcon)
	skipBtn.ConnectClicked(func() {
		if refs.Pomodoro != nil {
			refs.Pomodoro.Skip()
		}
	})

	controls.Append(startBtn)
	controls.Append(pauseBtn)
	controls.Append(resetBtn)
	controls.Append(skipBtn)
	box.Append(controls)

	b.Subscribe(bus.TopicPomodoro, func(e bus.Event) {
		ps, ok := e.Data.(state.PomodoroState)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			phaseLabel.SetText(fmtPhase(ps.Phase))
			timeLabel.SetText(formatDuration(ps.Remaining))
			sessionLabel.SetText(fmt.Sprintf("#%d", ps.SessionsCompleted))
		})
	})

	return box
}

func fmtPhase(phase string) string {
	switch phase {
	case "work":
		return "WORK"
	case "break":
		return "BREAK"
	default:
		return "IDLE"
	}
}

func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
