// Package lockscreen provides the lock screen surface.
package lockscreen

import (
	"os/exec"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// LockScreen is a full-screen lock surface.
type LockScreen struct {
	win *gtk.ApplicationWindow
	bus *bus.Bus
}

func New(app *gtk.Application, b *bus.Bus) *LockScreen {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-lockscreen",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeExclusive,
		ExclusiveZone: -1,
		Namespace:     "snry-lockscreen",
	})

	ls := &LockScreen{win: win, bus: b}
	ls.build()

	b.Subscribe(bus.TopicScreenLock, func(e bus.Event) {
		glib.IdleAdd(func() {
			locked := e.Data.(state.LockScreenState).Locked
			win.SetVisible(locked)
			if locked {
				win.GrabFocus()
			}
		})
	})

	win.SetVisible(false)
	return ls
}

func (ls *LockScreen) build() {
	root := gtk.NewBox(gtk.OrientationVertical, 0)
	root.AddCSSClass("lockscreen")
	root.SetHAlign(gtk.AlignCenter)
	root.SetVAlign(gtk.AlignCenter)

	clock := gtk.NewLabel("")
	clock.AddCSSClass("lockscreen-clock")
	clock.SetHAlign(gtk.AlignCenter)

	date := gtk.NewLabel("")
	date.AddCSSClass("lockscreen-date")
	date.SetHAlign(gtk.AlignCenter)

	entry := gtk.NewEntry()
	entry.AddCSSClass("lockscreen-entry")
	entry.SetVisibility(false)
	entry.SetInputPurpose(gtk.InputPurposePassword)
	entry.SetHAlign(gtk.AlignCenter)

	unlockBtn := gtkutil.M3FilledButton("Unlock", "lockscreen-unlock-btn")
	unlockBtn.SetHAlign(gtk.AlignCenter)

	root.Append(clock)
	root.Append(date)
	root.Append(entry)
	root.Append(unlockBtn)
	ls.win.SetChild(root)

	update := func() {
		clock.SetText(time.Now().Format("15:04"))
		date.SetText(time.Now().Format("Monday, January 02"))
	}
	update()
	glib.TimeoutAdd(1000, func() bool { update(); return true })

	entry.ConnectActivate(func() { ls.unlock(entry, unlockBtn) })
	unlockBtn.ConnectClicked(func() { ls.unlock(entry, unlockBtn) })
}

func (ls *LockScreen) unlock(entry *gtk.Entry, btn *gtk.Button) {
	pw := entry.Text()
	if pw == "" {
		return
	}

	// Run PAM auth in a goroutine to avoid blocking the UI.
	go func() {
		err := exec.Command("checkpw", "-P", pw).Run()
		glib.IdleAdd(func() {
			if err != nil {
				entry.AddCSSClass("error")
				glib.TimeoutAdd(350, func() bool {
					entry.RemoveCSSClass("error")
					return false
				})
				return
			}
			ls.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: false})
			entry.SetText("")
		})
	}()
}
