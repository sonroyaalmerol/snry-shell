package controlpanel

import (
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/controlsocket"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
)

// shellConfigProvider implements ConfigProvider for snry-shell settings
type shellConfigProvider struct {
	cfg *settings.Config
}

func newShellConfigProvider(cfg *settings.Config) *shellConfigProvider {
	return &shellConfigProvider{cfg: cfg}
}

func (s *shellConfigProvider) Name() string {
	return "Shell"
}

func (s *shellConfigProvider) Icon() string {
	return "tune"
}

func (s *shellConfigProvider) Load() error {
	if cfg, err := settings.Load(); err == nil {
		*s.cfg = cfg
	}
	return nil
}

func (s *shellConfigProvider) Save() error {
	if err := settings.Save(*s.cfg); err != nil {
		return err
	}
	// Notify the shell to reload settings
	s.notifyShellReload()
	return nil
}

func (s *shellConfigProvider) notifyShellReload() {
	// Send command to shell via control socket
	conn, err := net.Dial("unix", controlsocket.DefaultPath)
	if err != nil {
		// Shell might not be running, that's ok
		return
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("reload-settings")); err != nil {
		log.Printf("[CONTROLPANEL] notify shell reload: %v", err)
	}
}

func (s *shellConfigProvider) BuildWidget() gtk.Widgetter {
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("popup-scroll")
	scroll.SetVExpand(true)
	scroll.SetHExpand(true)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("settings-stack")
	box.SetVExpand(true)
	box.SetHExpand(true)

	// Settings sections
	box.Append(s.buildAppearanceSection())
	box.Append(s.buildBehaviorSection())

	scroll.SetChild(box)
	return scroll
}

func (s *shellConfigProvider) buildAppearanceSection() gtk.Widgetter {
	section := gtk.NewBox(gtk.OrientationVertical, 12)
	section.AddCSSClass("settings-page")

	// Section title
	title := gtk.NewLabel("Appearance")
	title.AddCSSClass("settings-label")
	title.SetHAlign(gtk.AlignStart)
	section.Append(title)

	// Card container using system-controls style
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("system-controls")

	// Dark mode toggle - use m3-switch style
	darkModeRow := s.buildSwitchRow("Dark Mode", "Use dark theme", s.cfg.DarkMode, func(active bool) {
		s.cfg.DarkMode = active
		if err := s.Save(); err != nil {
			log.Printf("[CONTROLPANEL] save dark mode: %v", err)
		}
	})
	card.Append(darkModeRow)

	section.Append(card)
	return section
}

func (s *shellConfigProvider) buildBehaviorSection() gtk.Widgetter {
	section := gtk.NewBox(gtk.OrientationVertical, 12)
	section.AddCSSClass("settings-page")
	section.SetMarginTop(24)

	// Section title
	title := gtk.NewLabel("Behavior")
	title.AddCSSClass("settings-label")
	title.SetHAlign(gtk.AlignStart)
	section.Append(title)

	// Card container
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("system-controls")

	// Do Not Disturb toggle
	dndRow := s.buildSwitchRow("Do Not Disturb", "Silence notifications", s.cfg.DoNotDisturb, func(active bool) {
		s.cfg.DoNotDisturb = active
		if err := s.Save(); err != nil {
			log.Printf("[CONTROLPANEL] save do-not-disturb: %v", err)
		}
	})
	card.Append(dndRow)

	// Separator
	card.Append(gtkutil.M3Divider())

	// Input mode dropdown
	inputModeRow := s.buildDropdownRow("Input Mode", "Touch input handling", []string{"auto", "tablet", "desktop"}, s.cfg.InputMode, func(value string) {
		s.cfg.InputMode = value
		if err := s.Save(); err != nil {
			log.Printf("[CONTROLPANEL] save input mode: %v", err)
		}
	})
	card.Append(inputModeRow)

	section.Append(card)

	// ── Idle / Lock section ────────────────────────────────────────────────
	idleSection := s.buildIdleSection()
	section.Append(idleSection)

	return section
}

func (s *shellConfigProvider) buildIdleSection() gtk.Widgetter {
	section := gtk.NewBox(gtk.OrientationVertical, 12)
	section.AddCSSClass("settings-page")
	section.SetMarginTop(24)

	title := gtk.NewLabel("Idle & Lock")
	title.AddCSSClass("settings-label")
	title.SetHAlign(gtk.AlignStart)
	section.Append(title)

	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("system-controls")

	// Idle lock timeout (minutes; 0 = disabled).
	lockTimeoutRow := s.buildSpinRow(
		"Lock timeout", "Minutes of inactivity before locking (0 = disabled)",
		0, 120, s.cfg.IdleLockTimeout/60,
		func(v int) {
			s.cfg.IdleLockTimeout = v * 60
			if err := s.Save(); err != nil {
				log.Printf("[CONTROLPANEL] save idle lock timeout: %v", err)
			}
		},
	)
	card.Append(lockTimeoutRow)
	card.Append(gtkutil.M3Divider())

	// Suspend timeout (minutes after lock; 0 = disabled).
	suspendTimeoutRow := s.buildSpinRow(
		"Suspend after lock", "Extra minutes after locking before suspend (0 = disabled)",
		0, 120, s.cfg.IdleSuspendTimeout/60,
		func(v int) {
			s.cfg.IdleSuspendTimeout = v * 60
			if err := s.Save(); err != nil {
				log.Printf("[CONTROLPANEL] save idle suspend timeout: %v", err)
			}
		},
	)
	card.Append(suspendTimeoutRow)

	section.Append(card)

	// Lock Screen settings card
	lockCard := gtk.NewBox(gtk.OrientationVertical, 0)
	lockCard.AddCSSClass("system-controls")
	lockCard.SetMarginTop(16)

	// Max attempts before lockout
	maxAttemptsRow := s.buildSpinRow(
		"Max password attempts", "Attempts before temporary lockout",
		1, 10, s.cfg.LockMaxAttempts,
		func(v int) {
			s.cfg.LockMaxAttempts = v
			if err := s.Save(); err != nil {
				log.Printf("[CONTROLPANEL] save lock max attempts: %v", err)
			}
		},
	)
	lockCard.Append(maxAttemptsRow)
	lockCard.Append(gtkutil.M3Divider())

	// Lockout duration (seconds)
	lockoutDurationRow := s.buildSpinRow(
		"Lockout duration", "Seconds to lock out after max attempts",
		5, 300, s.cfg.LockoutDuration,
		func(v int) {
			s.cfg.LockoutDuration = v
			if err := s.Save(); err != nil {
				log.Printf("[CONTROLPANEL] save lockout duration: %v", err)
			}
		},
	)
	lockCard.Append(lockoutDurationRow)
	lockCard.Append(gtkutil.M3Divider())

	// Show clock toggle
	showClockRow := s.buildSwitchRow("Show clock", "Display clock on lockscreen", s.cfg.LockShowClock, func(active bool) {
		s.cfg.LockShowClock = active
		if err := s.Save(); err != nil {
			log.Printf("[CONTROLPANEL] save lock show clock: %v", err)
		}
	})
	lockCard.Append(showClockRow)
	lockCard.Append(gtkutil.M3Divider())

	// Show user toggle
	showUserRow := s.buildSwitchRow("Show username", "Display username on lockscreen", s.cfg.LockShowUser, func(active bool) {
		s.cfg.LockShowUser = active
		if err := s.Save(); err != nil {
			log.Printf("[CONTROLPANEL] save lock show user: %v", err)
		}
	})
	lockCard.Append(showUserRow)

	section.Append(lockCard)
	return section
}

// buildSpinRow creates a row with a label, subtitle, and a spin-button for
// an integer value in [min, max].
func (s *shellConfigProvider) buildSpinRow(title, subtitle string, min, max, current int, callback func(int)) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 16)
	row.AddCSSClass("m3-switch-row")

	textBox := gtk.NewBox(gtk.OrientationVertical, 4)
	textBox.SetHExpand(true)

	titleLabel := gtk.NewLabel(title)
	titleLabel.AddCSSClass("m3-switch-row-label")
	titleLabel.SetHAlign(gtk.AlignStart)
	textBox.Append(titleLabel)

	if subtitle != "" {
		sub := gtk.NewLabel(subtitle)
		sub.AddCSSClass("m3-switch-row-sublabel")
		sub.SetHAlign(gtk.AlignStart)
		textBox.Append(sub)
	}
	row.Append(textBox)

	// Simple editable entry acting as a spin box.
	entry := gtk.NewEntry()
	entry.AddCSSClass("settings-spin-entry")
	entry.SetText(fmt.Sprintf("%d", current))
	entry.SetMaxWidthChars(4)
	entry.SetHAlign(gtk.AlignEnd)

	entry.ConnectActivate(func() {
		v, err := strconv.Atoi(entry.Text())
		if err != nil || v < min || v > max {
			entry.SetText(fmt.Sprintf("%d", current))
			return
		}
		current = v
		callback(v)
	})
	// Also save on focus-out.
	focusCtrl := gtk.NewEventControllerFocus()
	focusCtrl.ConnectLeave(func() {
		v, err := strconv.Atoi(entry.Text())
		if err != nil || v < min || v > max {
			entry.SetText(fmt.Sprintf("%d", current))
			return
		}
		if v != current {
			current = v
			callback(v)
		}
	})
	entry.AddController(focusCtrl)

	row.Append(entry)
	return row
}

func (s *shellConfigProvider) buildSwitchRow(title, subtitle string, active bool, callback func(bool)) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 16)
	row.AddCSSClass("m3-switch-row")

	// Text content
	textBox := gtk.NewBox(gtk.OrientationVertical, 4)
	textBox.SetHExpand(true)

	titleLabel := gtk.NewLabel(title)
	titleLabel.AddCSSClass("m3-switch-row-label")
	titleLabel.SetHAlign(gtk.AlignStart)
	textBox.Append(titleLabel)

	if subtitle != "" {
		subtitleLabel := gtk.NewLabel(subtitle)
		subtitleLabel.AddCSSClass("m3-switch-row-sublabel")
		subtitleLabel.SetHAlign(gtk.AlignStart)
		textBox.Append(subtitleLabel)
	}

	row.Append(textBox)

	// Use m3-switch class from shell
	switchBtn := gtk.NewSwitch()
	switchBtn.AddCSSClass("m3-switch")
	switchBtn.SetActive(active)
	switchBtn.ConnectStateSet(func(state bool) bool {
		callback(state)
		return false
	})
	row.Append(switchBtn)

	return row
}

func (s *shellConfigProvider) buildDropdownRow(title, subtitle string, options []string, current string, callback func(string)) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 16)
	row.AddCSSClass("m3-switch-row")

	// Text content
	textBox := gtk.NewBox(gtk.OrientationVertical, 4)
	textBox.SetHExpand(true)

	titleLabel := gtk.NewLabel(title)
	titleLabel.AddCSSClass("m3-switch-row-label")
	titleLabel.SetHAlign(gtk.AlignStart)
	textBox.Append(titleLabel)

	if subtitle != "" {
		subtitleLabel := gtk.NewLabel(subtitle)
		subtitleLabel.AddCSSClass("m3-switch-row-sublabel")
		subtitleLabel.SetHAlign(gtk.AlignStart)
		textBox.Append(subtitleLabel)
	}

	row.Append(textBox)

	// Dropdown with settings-dropdown style
	dropdown := gtk.NewDropDownFromStrings(options)
	dropdown.AddCSSClass("settings-dropdown")

	// Set current value
	for i, opt := range options {
		if opt == current {
			dropdown.SetSelected(uint(i))
			break
		}
	}

	dropdown.Connect("notify::selected", func() {
		idx := dropdown.Selected()
		if idx < uint(len(options)) {
			callback(options[idx])
		}
	})

	row.Append(dropdown)

	return row
}
