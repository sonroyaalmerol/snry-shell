// Package lockscreen provides the full-screen lock surface. It replaces
// hyprlock. One window is created per connected monitor; all windows
// share the same authentication state so a failed attempt on one monitor
// is reflected on all others.
package lockscreen

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/msteinert/pam/v2"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
	"github.com/sonroyaalmerol/snry-shell/internal/store"
)

const (
	wallpaperStoreKey      = "theme.wallpaper"
	defaultMaxAttempts     = 3
	defaultLockoutDuration = 30 * time.Second
)

// LockScreen manages one lock window per monitor.
type LockScreen struct {
	app *gtk.Application
	bus *bus.Bus

	// shared auth state (lock-free via atomics)
	inAuth    atomic.Bool
	attempts  atomic.Int32
	lockedOut atomic.Bool

	// configurable settings (GTK main-thread only, no lock needed)
	maxAttempts     int
	lockoutDuration time.Duration
	showClock       bool
	showUser        bool
	clockFormat     string

	windows []*lockWindow
}

type lockWindow struct {
	win    *gtk.ApplicationWindow
	mon    *gdk.Monitor
	bg     *gtk.Picture
	clock  *gtk.Label
	date   *gtk.Label
	entry  *gtk.Entry
	status *gtk.Label
	unlock *gtk.Button
}

// New creates and returns the lockscreen manager. Windows are spawned per
// monitor at construction; monitor hotplug is handled internally.
func New(app *gtk.Application, b *bus.Bus) *LockScreen {
	ls := &LockScreen{
		app:             app,
		bus:             b,
		maxAttempts:     defaultMaxAttempts,
		lockoutDuration: defaultLockoutDuration,
		showClock:       true,
		showUser:        true,
		clockFormat:     "24h",
	}

	ls.refreshMonitors()

	// Watch for monitor hotplug.
	if d := gdk.DisplayGetDefault(); d != nil {
		d.Monitors().ConnectItemsChanged(func(_, _, _ uint) {
			glib.IdleAdd(ls.refreshMonitors)
		})
	}

	// Show / hide on lock state changes.
	b.Subscribe(bus.TopicScreenLock, func(e bus.Event) {
		locked, ok := e.Data.(state.LockScreenState)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			ls.setVisible(locked.Locked)
			if locked.Locked {
				ls.focusPrimary()
			} else {
				ls.resetAuth()
			}
		})
	})

	// Update wallpaper when theme regenerates.
	b.Subscribe(bus.TopicThemeChanged, func(e bus.Event) {
		glib.IdleAdd(ls.updateWallpaper)
	})

	// Listen for settings changes
	b.Subscribe(bus.TopicSettingsChanged, func(e bus.Event) {
		if cfg, ok := e.Data.(settings.Config); ok {
			glib.IdleAdd(func() {
				ls.UpdateSettings(cfg)
			})
		}
	})

	return ls
}

// UpdateSettings updates the lockscreen configuration
func (ls *LockScreen) UpdateSettings(cfg settings.Config) {
	ls.maxAttempts = max(cfg.LockMaxAttempts, 1)

	ls.lockoutDuration = max(time.Duration(cfg.LockoutDuration)*time.Second, 5*time.Second)

	ls.showClock = cfg.LockShowClock
	ls.showUser = cfg.LockShowUser
	ls.clockFormat = cfg.ClockFormat
}

// refreshMonitors tears down existing windows and creates one per connected
// monitor. Called at startup and on hotplug events.
func (ls *LockScreen) refreshMonitors() {
	for _, w := range ls.windows {
		w.win.Close()
	}
	ls.windows = nil

	d := gdk.DisplayGetDefault()
	if d == nil {
		return
	}
	mons := d.Monitors()
	for i := uint(0); i < mons.NItems(); i++ {
		item := mons.Item(i)
		if item == nil {
			continue
		}
		mon := &gdk.Monitor{Object: item}
		ls.windows = append(ls.windows, ls.newWindow(mon))
	}
}

func (ls *LockScreen) newWindow(mon *gdk.Monitor) *lockWindow {
	win := layershell.NewWindow(ls.app, layershell.WindowConfig{
		Name:          "snry-lockscreen",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeOnDemand,
		ExclusiveZone: -1,
		Namespace:     "snry-lockscreen",
		Monitor:       mon,
	})

	lw := &lockWindow{win: win, mon: mon}
	ls.buildWindow(lw)
	win.SetVisible(false)
	return lw
}

func (ls *LockScreen) buildWindow(lw *lockWindow) {
	// ── background ──────────────────────────────────────────────────────────
	overlay := gtk.NewOverlay()

	lw.bg = gtk.NewPicture()
	lw.bg.SetContentFit(gtk.ContentFitCover)
	lw.bg.SetCanShrink(true)
	lw.bg.SetHExpand(true)
	lw.bg.SetVExpand(true)
	lw.bg.AddCSSClass("lockscreen-bg")
	if wp := store.LookupOr(wallpaperStoreKey, ""); wp != "" {
		lw.bg.SetFilename(wp)
	}
	overlay.SetChild(lw.bg)

	dim := gtk.NewBox(gtk.OrientationVertical, 0)
	dim.AddCSSClass("lockscreen-dim")
	dim.SetHExpand(true)
	dim.SetVExpand(true)
	overlay.AddOverlay(dim)

	// ── main content ─────────────────────────────────────────────────────────
	mainContent := gtk.NewBox(gtk.OrientationVertical, 0)
	mainContent.SetHExpand(true)
	mainContent.SetVExpand(true)
	mainContent.SetMarginTop(80)
	mainContent.SetMarginBottom(80)
	mainContent.SetMarginStart(40)
	mainContent.SetMarginEnd(40)
	overlay.AddOverlay(mainContent)

	// 1. Clock & Date (Top Aligned)
	topBox := gtk.NewBox(gtk.OrientationVertical, 0)
	topBox.SetVAlign(gtk.AlignStart)
	topBox.SetHAlign(gtk.AlignStart)

	lw.clock = gtk.NewLabel("")
	lw.clock.AddCSSClass("lockscreen-clock")
	lw.clock.SetHAlign(gtk.AlignStart)

	lw.date = gtk.NewLabel("")
	lw.date.AddCSSClass("lockscreen-date")
	lw.date.SetHAlign(gtk.AlignStart)

	topBox.Append(lw.clock)
	topBox.Append(lw.date)
	mainContent.Append(topBox)

	// Spacer to push auth content to bottom
	spacer := gtk.NewBox(gtk.OrientationVertical, 0)
	spacer.SetVExpand(true)
	mainContent.Append(spacer)

	// 2. Auth content (Bottom Aligned)
	authBox := gtk.NewBox(gtk.OrientationVertical, 24)
	authBox.SetVAlign(gtk.AlignEnd)
	authBox.SetHAlign(gtk.AlignCenter)
	authBox.AddCSSClass("lockscreen-auth-box")

	// User info
	userRow := gtk.NewBox(gtk.OrientationVertical, 12)
	userRow.SetHAlign(gtk.AlignCenter)

	userIcon := gtkutil.MaterialIcon("account_circle", "lockscreen-user-icon")
	userIcon.SetSizeRequest(80, 80)

	username := currentUser()
	userLabel := gtk.NewLabel(username)
	userLabel.AddCSSClass("lockscreen-username")

	userRow.Append(userIcon)
	userRow.Append(userLabel)
	authBox.Append(userRow)

	// Password entry card
	entryCard := gtk.NewBox(gtk.OrientationVertical, 16)
	entryCard.AddCSSClass("lockscreen-entry-card")

	lw.entry = gtk.NewEntry()
	lw.entry.AddCSSClass("lockscreen-entry")
	lw.entry.SetVisibility(false)
	lw.entry.SetInputPurpose(gtk.InputPurposePassword)
	lw.entry.SetPlaceholderText("Enter Password")
	lw.entry.SetHAlign(gtk.AlignCenter)

	entryRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	entryRow.AddCSSClass("lockscreen-entry-row")
	entryRow.SetHAlign(gtk.AlignCenter)

	eyeBtn := gtkutil.M3IconButton("visibility_off", "lockscreen-eye-btn")
	gtkutil.AddPasswordToggle(lw.entry, eyeBtn)

	entryRow.Append(lw.entry)
	entryRow.Append(eyeBtn)
	entryCard.Append(entryRow)

	// Status label
	lw.status = gtk.NewLabel("")
	lw.status.AddCSSClass("lockscreen-status")
	lw.status.SetHAlign(gtk.AlignCenter)
	entryCard.Append(lw.status)

	authBox.Append(entryCard)

	// Unlock button & Actions
	actionsRow := gtk.NewBox(gtk.OrientationHorizontal, 16)
	actionsRow.SetHAlign(gtk.AlignCenter)

	lw.unlock = gtkutil.M3FilledButton("Unlock", "lockscreen-unlock-btn")
	lw.unlock.SetSizeRequest(120, -1)

	emergencyBtn := gtkutil.M3TextButton("Emergency", "lockscreen-action-btn")

	actionsRow.Append(emergencyBtn)
	actionsRow.Append(lw.unlock)
	authBox.Append(actionsRow)

	mainContent.Append(authBox)

	lw.win.SetChild(overlay)

	// Wire auth actions.
	lw.entry.ConnectActivate(func() { ls.unlock(lw) })
	lw.unlock.ConnectClicked(func() { ls.unlock(lw) })

	// Clock ticker.
	ls.startClock(lw)
}

func (ls *LockScreen) startClock(lw *lockWindow) {
	update := func() {
		now := time.Now()

		format := ls.clockFormat

		clockStr := "15:04"
		if format == "12h" {
			clockStr = "03:04"
		}

		lw.clock.SetText(now.Format(clockStr))
		lw.date.SetText(now.Format("Monday, January 02"))
	}
	update()
	glib.TimeoutAdd(1000, func() bool {
		if lw.win.Visible() {
			update()
		}
		return true
	})
}

// ── visibility ───────────────────────────────────────────────────────────────

func (ls *LockScreen) setVisible(visible bool) {
	for _, w := range ls.windows {
		w.win.SetVisible(visible)
		if visible {
			w.entry.SetText("")
			w.entry.RemoveCSSClass("error")
			w.entry.GrabFocus()
		}
	}
}

func (ls *LockScreen) focusPrimary() {
	if len(ls.windows) > 0 {
		ls.windows[0].win.GrabFocus()
		ls.windows[0].entry.GrabFocus()
	}
}

// ── wallpaper ────────────────────────────────────────────────────────────────

func (ls *LockScreen) updateWallpaper() {
	wp := store.LookupOr(wallpaperStoreKey, "")
	for _, w := range ls.windows {
		if wp != "" {
			w.bg.SetFilename(wp)
		}
	}
}

// ── authentication ───────────────────────────────────────────────────────────

func (ls *LockScreen) unlock(lw *lockWindow) {
	if ls.inAuth.Load() || ls.lockedOut.Load() {
		return
	}
	pw := lw.entry.Text()
	if pw == "" {
		return
	}
	ls.inAuth.Store(true)

	go func() {
		err := authenticate(pw)
		glib.IdleAdd(func() {
			ls.inAuth.Store(false)

			if err == nil {
				ls.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: false})
				return
			}

			attempts := ls.attempts.Add(1)

			// Shake all entries.
			for _, w := range ls.windows {
				w.entry.SetText("")
				w.entry.AddCSSClass("error")
				glib.TimeoutAdd(400, func() bool {
					w.entry.RemoveCSSClass("error")
					return false
				})
			}

			if int(attempts) >= ls.maxAttempts {
				ls.startLockout()
			} else {
				remaining := ls.maxAttempts - int(attempts)
				ls.setStatus(fmt.Sprintf("Incorrect password. %d attempt(s) remaining.", remaining))
			}
		})
	}()
}

func (ls *LockScreen) startLockout() {
	ls.lockedOut.Store(true)

	ls.setLockoutButtons(true)

	deadline := time.Now().Add(ls.lockoutDuration)
	glib.TimeoutAdd(1000, func() bool {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			ls.lockedOut.Store(false)
			ls.attempts.Store(0)
			ls.setStatus("")
			ls.setLockoutButtons(false)
			return false
		}
		ls.setStatus(fmt.Sprintf("Too many attempts. Try again in %ds.", int(remaining.Seconds())+1))
		return true
	})
}

func (ls *LockScreen) setLockoutButtons(disabled bool) {
	for _, w := range ls.windows {
		w.entry.SetSensitive(!disabled)
		w.unlock.SetSensitive(!disabled)
	}
}

func (ls *LockScreen) setStatus(msg string) {
	for _, w := range ls.windows {
		w.status.SetText(msg)
	}
}

func (ls *LockScreen) resetAuth() {
	ls.attempts.Store(0)
	ls.lockedOut.Store(false)
	ls.inAuth.Store(false)
	ls.setStatus("")
	ls.setLockoutButtons(false)
}

// ── PAM authentication ────────────────────────────────────────────────────────

func authenticate(password string) error {
	username := currentUser()
	log.Printf("[LOCKSCREEN] attempting PAM authentication for user %q", username)

	if err := authenticateWithPAM(username, password); err == nil {
		log.Printf("[LOCKSCREEN] PAM authentication succeeded for %q", username)
		go unlockKeyring(password)
		return nil
	} else {
		log.Printf("[LOCKSCREEN] PAM authentication failed: %v", err)
	}

	log.Printf("[LOCKSCREEN] falling back to su authentication")
	if err := authenticateWithSu(username, password); err == nil {
		log.Printf("[LOCKSCREEN] su authentication succeeded for %q", username)
		go unlockKeyring(password)
		return nil
	} else {
		log.Printf("[LOCKSCREEN] su failed: %v", err)
	}

	log.Printf("[LOCKSCREEN] all auth methods failed for %q", username)
	return fmt.Errorf("authentication failed")
}

func authenticateWithPAM(username, password string) error {
	trans, err := pam.StartFunc("login", username, func(s pam.Style, msg string) (string, error) {
		switch s {
		case pam.PromptEchoOff:
			return password, nil
		case pam.PromptEchoOn:
			return username, nil
		case pam.ErrorMsg:
			log.Printf("[LOCKSCREEN] PAM error: %s", msg)
			return "", nil
		case pam.TextInfo:
			log.Printf("[LOCKSCREEN] PAM info: %s", msg)
			return "", nil
		default:
			return "", fmt.Errorf("unsupported PAM message style: %v", s)
		}
	})
	if err != nil {
		return fmt.Errorf("PAM start failed: %w", err)
	}
	defer trans.End()

	err = trans.Authenticate(0)
	if err != nil {
		return fmt.Errorf("PAM authentication failed: %w", err)
	}

	err = trans.AcctMgmt(0)
	if err != nil {
		return fmt.Errorf("PAM account check failed: %w", err)
	}

	return nil
}

func authenticateWithSu(username, password string) error {
	cmd := exec.Command("su", username, "-c", "true")
	cmd.Stdin = strings.NewReader(password + "\n")
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("su auth failed: %w", err)
	}
	return nil
}

func unlockKeyring(password string) {
	cmd := exec.Command("secret-tool", "store", "--label=snry-shell", "service", "snry-shell")
	cmd.Stdin = strings.NewReader(password)
	if err := cmd.Run(); err == nil {
		exec.Command("secret-tool", "clear", "service", "snry-shell").Run()
		log.Printf("[LOCKSCREEN] keyring unlocked successfully")
		return
	}

	cmd = exec.Command("gnome-keyring-daemon", "--unlock")
	cmd.Stdin = strings.NewReader(password + "\n")
	if err := cmd.Run(); err == nil {
		log.Printf("[LOCKSCREEN] GNOME keyring unlocked via daemon")
		return
	}

	cmd = exec.Command("dbus-send", "--session", "--dest=org.gnome.keyring", "--type=method_call",
		"/org/freedesktop/portal/desktop", "org.freedesktop.portal.Settings.Read", "string:org.freedesktop.appearance", "string:color-scheme")
	if err := cmd.Run(); err == nil {
		log.Printf("[LOCKSCREEN] GNOME keyring unlocked via D-Bus")
		return
	}

	log.Printf("[LOCKSCREEN] could not unlock keyring (may not be running or not supported)")
}

func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u := os.Getenv("LOGNAME"); u != "" {
		return u
	}
	return "user"
}
