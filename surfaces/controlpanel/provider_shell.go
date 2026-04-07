package controlpanel

import (
	"log"
	"net"

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
	s.notifyShellReload()
	return nil
}

func (s *shellConfigProvider) notifyShellReload() {
	conn, err := net.Dial("unix", controlsocket.DefaultPath)
	if err != nil {
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

	box.Append(s.buildAppearanceSection())
	box.Append(s.buildBehaviorSection())

	scroll.SetChild(box)
	return scroll
}

func (s *shellConfigProvider) buildAppearanceSection() gtk.Widgetter {
	darkModeRow, _ := gtkutil.SwitchRowFull("Dark Mode", "Use dark theme", s.cfg.DarkMode, func(active bool) {
		s.cfg.DarkMode = active
		if err := s.Save(); err != nil {
			log.Printf("[CONTROLPANEL] save dark mode: %v", err)
		}
	})
	return gtkutil.SettingsSection("Appearance", darkModeRow)
}

func (s *shellConfigProvider) buildBehaviorSection() gtk.Widgetter {
	outer := gtk.NewBox(gtk.OrientationVertical, 0)

	dndRow, _ := gtkutil.SwitchRowFull("Do Not Disturb", "Silence notifications", s.cfg.DoNotDisturb, func(active bool) {
		s.cfg.DoNotDisturb = active
		if err := s.Save(); err != nil {
			log.Printf("[CONTROLPANEL] save do-not-disturb: %v", err)
		}
	})

	inputModeRow := gtkutil.DropdownRow("Input Mode", "Touch input handling",
		[]string{"auto", "tablet", "desktop"}, s.cfg.InputMode,
		func(value string) {
			s.cfg.InputMode = value
			if err := s.Save(); err != nil {
				log.Printf("[CONTROLPANEL] save input mode: %v", err)
			}
		})

	behaviorSection := gtkutil.SettingsSection("Behavior", dndRow, inputModeRow)
	behaviorSection.SetMarginTop(24)
	outer.Append(behaviorSection)

	idleSection := s.buildIdleSection()
	outer.Append(idleSection)

	return outer
}

func (s *shellConfigProvider) buildIdleSection() gtk.Widgetter {
	outer := gtk.NewBox(gtk.OrientationVertical, 0)

	lockTimeoutRow := gtkutil.SpinRow(
		"Lock timeout", "Minutes of inactivity before locking (0 = disabled)",
		0, 120, s.cfg.IdleLockTimeout/60,
		func(v int) {
			s.cfg.IdleLockTimeout = v * 60
			if err := s.Save(); err != nil {
				log.Printf("[CONTROLPANEL] save idle lock timeout: %v", err)
			}
		},
	)

	suspendTimeoutRow := gtkutil.SpinRow(
		"Suspend after lock", "Extra minutes after locking before suspend (0 = disabled)",
		0, 120, s.cfg.IdleSuspendTimeout/60,
		func(v int) {
			s.cfg.IdleSuspendTimeout = v * 60
			if err := s.Save(); err != nil {
				log.Printf("[CONTROLPANEL] save idle suspend timeout: %v", err)
			}
		},
	)

	idleSection := gtkutil.SettingsSection("Idle & Lock", lockTimeoutRow, suspendTimeoutRow)
	idleSection.SetMarginTop(24)
	outer.Append(idleSection)

	maxAttemptsRow := gtkutil.SpinRow(
		"Max password attempts", "Attempts before temporary lockout",
		1, 10, s.cfg.LockMaxAttempts,
		func(v int) {
			s.cfg.LockMaxAttempts = v
			if err := s.Save(); err != nil {
				log.Printf("[CONTROLPANEL] save lock max attempts: %v", err)
			}
		},
	)

	lockoutDurationRow := gtkutil.SpinRow(
		"Lockout duration", "Seconds to lock out after max attempts",
		5, 300, s.cfg.LockoutDuration,
		func(v int) {
			s.cfg.LockoutDuration = v
			if err := s.Save(); err != nil {
				log.Printf("[CONTROLPANEL] save lockout duration: %v", err)
			}
		},
	)

	showClockRow, _ := gtkutil.SwitchRowFull("Show clock", "Display clock on lockscreen", s.cfg.LockShowClock, func(active bool) {
		s.cfg.LockShowClock = active
		if err := s.Save(); err != nil {
			log.Printf("[CONTROLPANEL] save lock show clock: %v", err)
		}
	})

	showUserRow, _ := gtkutil.SwitchRowFull("Show username", "Display username on lockscreen", s.cfg.LockShowUser, func(active bool) {
		s.cfg.LockShowUser = active
		if err := s.Save(); err != nil {
			log.Printf("[CONTROLPANEL] save lock show user: %v", err)
		}
	})

	lockSection := gtkutil.SettingsSection("", maxAttemptsRow, lockoutDurationRow, showClockRow, showUserRow)
	lockSection.SetMarginTop(16)
	outer.Append(lockSection)

	return outer
}
