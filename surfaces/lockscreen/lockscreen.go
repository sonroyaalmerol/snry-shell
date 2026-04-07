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
	"sync"
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

	// shared auth state
	mu        sync.Mutex
	inAuth    bool
	attempts  int
	lockedOut bool

	// configurable settings
	maxAttempts     int
	lockoutDuration time.Duration
	showClock       bool
	showUser        bool

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
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.maxAttempts = cfg.LockMaxAttempts
	if ls.maxAttempts < 1 {
		ls.maxAttempts = 1
	}

	ls.lockoutDuration = time.Duration(cfg.LockoutDuration) * time.Second
	if ls.lockoutDuration < 5*time.Second {
		ls.lockoutDuration = 5 * time.Second
	}

	ls.showClock = cfg.LockShowClock
	ls.showUser = cfg.LockShowUser
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
		KeyboardMode:  layershell.KeyboardModeExclusive,
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

	// Dark dim layer for readability.
	// Note: Input blocking is handled by layer-shell's KeyboardModeExclusive,
	// so we don't need click capture here (which would block the OSK).
	dim := gtk.NewBox(gtk.OrientationVertical, 0)
	dim.AddCSSClass("lockscreen-dim")
	dim.SetHExpand(true)
	dim.SetVExpand(true)
	overlay.AddOverlay(dim)

	// ── auth card ────────────────────────────────────────────────────────────
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("lockscreen-card")
	card.SetHAlign(gtk.AlignCenter)
	card.SetVAlign(gtk.AlignCenter)
	overlay.AddOverlay(card)
	overlay.SetMeasureOverlay(card, true)

	// Clock.
	lw.clock = gtk.NewLabel("")
	lw.clock.AddCSSClass("lockscreen-clock")
	lw.clock.SetHAlign(gtk.AlignCenter)

	// Date.
	lw.date = gtk.NewLabel("")
	lw.date.AddCSSClass("lockscreen-date")
	lw.date.SetHAlign(gtk.AlignCenter)

	// User identity row.
	userRow := gtk.NewBox(gtk.OrientationHorizontal, 12)
	userRow.AddCSSClass("lockscreen-user-row")
	userRow.SetHAlign(gtk.AlignCenter)

	userIcon := gtkutil.MaterialIcon("account_circle", "lockscreen-user-icon")
	username := currentUser()
	userLabel := gtk.NewLabel(username)
	userLabel.AddCSSClass("lockscreen-username")
	userRow.Append(userIcon)
	userRow.Append(userLabel)

	// Password entry.
	lw.entry = gtk.NewEntry()
	lw.entry.AddCSSClass("lockscreen-entry")
	lw.entry.SetVisibility(false)
	lw.entry.SetInputPurpose(gtk.InputPurposePassword)
	lw.entry.SetPlaceholderText("Password")
	lw.entry.SetHAlign(gtk.AlignCenter)

	// Eye toggle button inside/next to entry.
	entryRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	entryRow.AddCSSClass("lockscreen-entry-row")
	entryRow.SetHAlign(gtk.AlignCenter)
	eyeBtn := gtkutil.M3IconButton("visibility_off", "lockscreen-eye-btn")
	eyeBtn.ConnectClicked(func() {
		if lw.entry.Visibility() {
			lw.entry.SetVisibility(false)
			eyeBtn.SetChild(gtkutil.MaterialIcon("visibility_off"))
		} else {
			lw.entry.SetVisibility(true)
			eyeBtn.SetChild(gtkutil.MaterialIcon("visibility"))
		}
	})
	entryRow.Append(lw.entry)
	entryRow.Append(eyeBtn)

	// Status / error label.
	lw.status = gtk.NewLabel("")
	lw.status.AddCSSClass("lockscreen-status")
	lw.status.SetHAlign(gtk.AlignCenter)

	// Unlock button.
	lw.unlock = gtkutil.M3FilledButton("Unlock", "lockscreen-unlock-btn")
	lw.unlock.SetHAlign(gtk.AlignCenter)

	card.Append(lw.clock)
	card.Append(lw.date)
	card.Append(userRow)
	card.Append(entryRow)
	card.Append(lw.status)
	card.Append(lw.unlock)

	lw.win.SetChild(overlay)

	// Wire auth actions.
	lw.entry.ConnectActivate(func() { ls.unlock(lw) })
	lw.unlock.ConnectClicked(func() { ls.unlock(lw) })

	// Clock ticker (shared tick for all windows via IdleAdd).
	ls.startClock(lw)
}

func (ls *LockScreen) startClock(lw *lockWindow) {
	update := func() {
		now := time.Now()
		lw.clock.SetText(now.Format("15:04"))
		lw.date.SetText(now.Format("Monday, January 02"))
	}
	update()
	glib.TimeoutAdd(1000, func() bool {
		if lw.win.Visible() || true { // always tick so clock is correct on show
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
	ls.mu.Lock()
	if ls.inAuth || ls.lockedOut {
		ls.mu.Unlock()
		return
	}
	pw := lw.entry.Text()
	if pw == "" {
		ls.mu.Unlock()
		return
	}
	ls.inAuth = true
	ls.mu.Unlock()

	go func() {
		err := authenticate(pw)
		glib.IdleAdd(func() {
			ls.mu.Lock()
			ls.inAuth = false
			ls.mu.Unlock()

			if err == nil {
				ls.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: false})
				return
			}

			ls.mu.Lock()
			ls.attempts++
			attempts := ls.attempts
			ls.mu.Unlock()

			// Shake all entries.
			for _, w := range ls.windows {
				w.entry.SetText("")
				w.entry.AddCSSClass("error")
				glib.TimeoutAdd(400, func() bool {
					w.entry.RemoveCSSClass("error")
					return false
				})
			}

			if attempts >= ls.maxAttempts {
				ls.startLockout()
			} else {
				remaining := ls.maxAttempts - attempts
				ls.setStatus(fmt.Sprintf("Incorrect password. %d attempt(s) remaining.", remaining))
			}
		})
	}()
}

func (ls *LockScreen) startLockout() {
	ls.mu.Lock()
	ls.lockedOut = true
	ls.mu.Unlock()

	ls.setLockoutButtons(true)

	deadline := time.Now().Add(ls.lockoutDuration)
	glib.TimeoutAdd(1000, func() bool {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			ls.mu.Lock()
			ls.lockedOut = false
			ls.attempts = 0
			ls.mu.Unlock()
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
	ls.mu.Lock()
	ls.attempts = 0
	ls.lockedOut = false
	ls.inAuth = false
	ls.mu.Unlock()
	ls.setStatus("")
	ls.setLockoutButtons(false)
}

// ── PAM authentication ────────────────────────────────────────────────────────

// authenticate checks the given password using PAM (Pluggable Authentication Modules).
// This is the same authentication mechanism used by login, sudo, etc.
// On success, it also unlocks the default keyring with the same password.
func authenticate(password string) error {
	username := currentUser()
	log.Printf("[LOCKSCREEN] attempting PAM authentication for user %q", username)

	// Use direct PAM authentication (no external dependencies)
	if err := authenticateWithPAM(username, password); err == nil {
		log.Printf("[LOCKSCREEN] PAM authentication succeeded for %q", username)
		// Unlock keyring with the same password
		go unlockKeyring(password)
		return nil
	} else {
		log.Printf("[LOCKSCREEN] PAM authentication failed: %v", err)
	}

	// Fallback to su method if PAM fails
	log.Printf("[LOCKSCREEN] falling back to su authentication")
	if err := authenticateWithSu(username, password); err == nil {
		log.Printf("[LOCKSCREEN] su authentication succeeded for %q", username)
		// Unlock keyring with the same password
		go unlockKeyring(password)
		return nil
	} else {
		log.Printf("[LOCKSCREEN] su failed: %v", err)
	}

	log.Printf("[LOCKSCREEN] all auth methods failed for %q", username)
	return fmt.Errorf("authentication failed")
}

// authenticateWithPAM uses direct PAM authentication via the PAM library.
// This requires no external binaries and uses the system's PAM configuration.
func authenticateWithPAM(username, password string) error {
	// Create PAM transaction for the "login" service using StartFunc
	// This uses the same PAM stack as the system login
	trans, err := pam.StartFunc("login", username, func(s pam.Style, msg string) (string, error) {
		switch s {
		case pam.PromptEchoOff:
			// Password prompt - return the password
			return password, nil
		case pam.PromptEchoOn:
			// Username prompt (shouldn't happen since we provided it)
			return username, nil
		case pam.ErrorMsg:
			// Error message from PAM
			log.Printf("[LOCKSCREEN] PAM error: %s", msg)
			return "", nil
		case pam.TextInfo:
			// Informational message
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

	// Authenticate the user
	err = trans.Authenticate(0)
	if err != nil {
		return fmt.Errorf("PAM authentication failed: %w", err)
	}

	// Check account validity
	err = trans.AcctMgmt(0)
	if err != nil {
		return fmt.Errorf("PAM account check failed: %w", err)
	}

	return nil
}

// authenticateWithSu uses su as a fallback authentication method.
// It attempts to switch to the user with the provided password.
func authenticateWithSu(username, password string) error {
	// su without - reads password from stdin when not running from a terminal
	cmd := exec.Command("su", username, "-c", "true")
	cmd.Stdin = strings.NewReader(password + "\n")
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("su auth failed: %w", err)
	}
	return nil
}

// unlockKeyring attempts to unlock the default keyring using the provided password.
// It supports GNOME Keyring and other Secret Service implementations.
func unlockKeyring(password string) {
	// Try to unlock via secret-tool (libsecret CLI)
	// This works with GNOME Keyring and other Secret Service providers
	cmd := exec.Command("secret-tool", "store", "--label=snry-shell", "service", "snry-shell")
	cmd.Stdin = strings.NewReader(password)
	if err := cmd.Run(); err == nil {
		// Clean up the test entry
		exec.Command("secret-tool", "clear", "service", "snry-shell").Run()
		log.Printf("[LOCKSCREEN] keyring unlocked successfully")
		return
	}

	// Fallback: try gnome-keyring-daemon unlock
	cmd = exec.Command("gnome-keyring-daemon", "--unlock")
	cmd.Stdin = strings.NewReader(password + "\n")
	if err := cmd.Run(); err == nil {
		log.Printf("[LOCKSCREEN] GNOME keyring unlocked via daemon")
		return
	}

	// Try dbus-send method for GNOME Keyring
	cmd = exec.Command("dbus-send", "--session", "--dest=org.gnome.keyring", "--type=method_call",
		"/org/gnome/keyring/daemon", "org.gnome.keyringDaemon.Unlock", "string:"+password)
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
