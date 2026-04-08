package controlpanel

import (
	"log"
	"net"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/controlsocket"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
)

// baseShellProvider provides common functionality for shell providers
type baseShellProvider struct {
	cfg *settings.Config
}

func (b *baseShellProvider) Load() error {
	if cfg, err := settings.Load(); err == nil {
		*b.cfg = cfg
	}
	return nil
}

func (b *baseShellProvider) Save() error {
	if err := settings.Save(*b.cfg); err != nil {
		return err
	}
	b.notifyShellReload()
	return nil
}

func (b *baseShellProvider) notifyShellReload() {
	conn, err := net.Dial("unix", controlsocket.DefaultPath)
	if err != nil {
		return
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("reload-settings")); err != nil {
		log.Printf("[CONTROLPANEL] notify shell reload: %v", err)
	}
}

// appearanceConfigProvider handles appearance settings
type appearanceConfigProvider struct {
	baseShellProvider
}

func newAppearanceConfigProvider(cfg *settings.Config) *appearanceConfigProvider {
	return &appearanceConfigProvider{baseShellProvider{cfg: cfg}}
}

func (a *appearanceConfigProvider) Name() string { return "Appearance" }
func (a *appearanceConfigProvider) Icon() string { return "palette" }

func (a *appearanceConfigProvider) BuildWidget() gtk.Widgetter {
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("popup-scroll")

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("settings-stack")

	// Theme Section
	darkModeRow, _ := gtkutil.SwitchRowFull("Dark Mode", "Use dark theme", a.cfg.DarkMode, func(active bool) {
		a.cfg.DarkMode = active
		a.Save()
	})

	blurStrengthRow := gtkutil.SpinRow("Blur Strength", "Background blur intensity (0-100)", 0, 100, a.cfg.BlurStrength, func(v int) {
		a.cfg.BlurStrength = v
		a.Save()
	})

	box.Append(gtkutil.SettingsSection("Theme", darkModeRow, blurStrengthRow))

	// Bar Section
	barPosRow := gtkutil.DropdownRow("Bar Position", "Where the status bar is anchored",
		[]string{"top", "bottom"}, a.cfg.BarPosition, func(v string) {
			a.cfg.BarPosition = v
			a.Save()
		})

	showBatteryRow, _ := gtkutil.SwitchRowFull("Show Battery Percentage", "Display numeric battery level", a.cfg.BarShowBatteryPct, func(active bool) {
		a.cfg.BarShowBatteryPct = active
		a.Save()
	})

	box.Append(gtkutil.SettingsSection("Status Bar", barPosRow, showBatteryRow))

	// Clock Section
	clockFormatRow := gtkutil.DropdownRow("Clock Format", "Time display style",
		[]string{"12h", "24h"}, a.cfg.ClockFormat, func(v string) {
			a.cfg.ClockFormat = v
			a.Save()
		})

	box.Append(gtkutil.SettingsSection("Clock", clockFormatRow))

	scroll.SetChild(box)
	return scroll
}

// behaviorConfigProvider handles behavior settings
type behaviorConfigProvider struct {
	baseShellProvider
}

func newBehaviorConfigProvider(cfg *settings.Config) *behaviorConfigProvider {
	return &behaviorConfigProvider{baseShellProvider{cfg: cfg}}
}

func (b *behaviorConfigProvider) Name() string { return "Behavior" }
func (b *behaviorConfigProvider) Icon() string { return "tune" }

func (b *behaviorConfigProvider) BuildWidget() gtk.Widgetter {
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("popup-scroll")

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("settings-stack")

	// Interaction Section
	dndRow, _ := gtkutil.SwitchRowFull("Do Not Disturb", "Silence notifications", b.cfg.DoNotDisturb, func(active bool) {
		b.cfg.DoNotDisturb = active
		b.Save()
	})

	inputModeRow := gtkutil.DropdownRow("Input Mode", "Touch input handling",
		[]string{"auto", "tablet", "desktop"}, b.cfg.InputMode,
		func(value string) {
			b.cfg.InputMode = value
			b.Save()
		})

	box.Append(gtkutil.SettingsSection("Interaction", dndRow, inputModeRow))

	// Notifications Section
	notifTimeoutRow := gtkutil.SpinRow("Notification Timeout", "Milliseconds before toast disappears",
		1000, 30000, b.cfg.NotificationTimeout, func(v int) {
			b.cfg.NotificationTimeout = v
			b.Save()
		})

	notifPosRow := gtkutil.DropdownRow("Notification Position", "Where toasts appear",
		[]string{"top-right", "top-left", "bottom-right", "bottom-left"}, b.cfg.NotificationPosition,
		func(v string) {
			b.cfg.NotificationPosition = v
			b.Save()
		})

	box.Append(gtkutil.SettingsSection("Notifications", notifTimeoutRow, notifPosRow))

	scroll.SetChild(box)
	return scroll
}

// systemConfigProvider handles system, idle and lock settings
type systemConfigProvider struct {
	baseShellProvider
}

func newSystemConfigProvider(cfg *settings.Config) *systemConfigProvider {
	return &systemConfigProvider{baseShellProvider{cfg: cfg}}
}

func (s *systemConfigProvider) Name() string { return "System" }
func (s *systemConfigProvider) Icon() string { return "settings" }

func (s *systemConfigProvider) BuildWidget() gtk.Widgetter {
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("popup-scroll")

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("settings-stack")

	// Audio & Brightness Section
	volStepRow := gtkutil.SpinRow("Volume Step", "Percentage change per increment (1-10%)",
		1, 10, int(s.cfg.VolumeStep*100), func(v int) {
			s.cfg.VolumeStep = float64(v) / 100.0
			s.Save()
		})

	brightStepRow := gtkutil.SpinRow("Brightness Step", "Percentage change per increment (1-10%)",
		1, 10, int(s.cfg.BrightnessStep*100), func(v int) {
			s.cfg.BrightnessStep = float64(v) / 100.0
			s.Save()
		})

	box.Append(gtkutil.SettingsSection("Hardware Steps", volStepRow, brightStepRow))

	// Idle & Lock Section
	lockTimeoutRow := gtkutil.SpinRow(
		"Lock timeout", "Minutes of inactivity before locking (0 = disabled)",
		0, 120, s.cfg.IdleLockTimeout/60,
		func(v int) {
			s.cfg.IdleLockTimeout = v * 60
			s.Save()
		},
	)

	displayOffTimeoutRow := gtkutil.SpinRow(
		"Turn display off after lock", "Extra minutes after locking before display turns off (0 = disabled)",
		0, 120, s.cfg.IdleDisplayOffTimeout/60,
		func(v int) {
			s.cfg.IdleDisplayOffTimeout = v * 60
			s.Save()
		},
	)

	suspendTimeoutRow := gtkutil.SpinRow(
		"Suspend after lock", "Extra minutes after locking before suspend (0 = disabled)",
		0, 120, s.cfg.IdleSuspendTimeout/60,
		func(v int) {
			s.cfg.IdleSuspendTimeout = v * 60
			s.Save()
		},
	)

	box.Append(gtkutil.SettingsSection("Idle & Lock", lockTimeoutRow, displayOffTimeoutRow, suspendTimeoutRow))

	// System Buttons Section
	lidActionRow := gtkutil.DropdownRow("Lid close action", "Action to take when laptop lid is closed",
		[]string{"ignore", "lock", "suspend"}, s.cfg.LidCloseAction,
		func(value string) {
			s.cfg.LidCloseAction = value
			s.Save()
		})

	powerActionRow := gtkutil.DropdownRow("Power button action", "Action to take when power button is pressed",
		[]string{"ignore", "lock", "shutdown", "session-menu"}, s.cfg.PowerButtonAction,
		func(value string) {
			s.cfg.PowerButtonAction = value
			s.Save()
		})

	box.Append(gtkutil.SettingsSection("System Buttons", lidActionRow, powerActionRow))

	// Password & Lockscreen Section
	maxAttemptsRow := gtkutil.SpinRow(
		"Max password attempts", "Attempts before temporary lockout",
		1, 10, s.cfg.LockMaxAttempts,
		func(v int) {
			s.cfg.LockMaxAttempts = v
			s.Save()
		},
	)

	lockoutDurationRow := gtkutil.SpinRow(
		"Lockout duration", "Seconds to lock out after max attempts",
		5, 300, s.cfg.LockoutDuration,
		func(v int) {
			s.cfg.LockoutDuration = v
			s.Save()
		},
	)

	showClockRow, _ := gtkutil.SwitchRowFull("Show clock", "Display clock on lockscreen", s.cfg.LockShowClock, func(active bool) {
		s.cfg.LockShowClock = active
		s.Save()
	})

	showUserRow, _ := gtkutil.SwitchRowFull("Show username", "Display username on lockscreen", s.cfg.LockShowUser, func(active bool) {
		s.cfg.LockShowUser = active
		s.Save()
	})

	box.Append(gtkutil.SettingsSection("Lockscreen Security", maxAttemptsRow, lockoutDurationRow, showClockRow, showUserRow))

	scroll.SetChild(box)
	return scroll
}
