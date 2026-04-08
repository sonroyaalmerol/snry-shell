package settings

import (
	"github.com/sonroyaalmerol/snry-shell/internal/store"
)

const (
	keyDarkMode              = "dark_mode"
	keyDoNotDisturb          = "do_not_disturb"
	keyInputMode             = "input_mode"
	keyIdleLockTimeout       = "idle_lock_timeout"
	keyIdleDisplayOffTimeout = "idle_displayoff_timeout"
	keyIdleSuspendTimeout    = "idle_suspend_timeout"
	keyLockMaxAttempts       = "lock_max_attempts"
	keyLockoutDuration       = "lockout_duration"
	keyLockShowClock         = "lock_show_clock"
	keyLockShowUser          = "lock_show_user"
	keyLidCloseAction        = "lid_close_action"
	keyPowerButtonAction     = "power_button_action"
	keyBarPosition           = "bar.position"
	keyBarShowBatteryPct     = "bar.show_battery_pct"
	keyClockFormat           = "clock.format"
	keyNotificationTimeout   = "notifications.timeout"
	keyNotificationPosition  = "notifications.position"
	keyVolumeStep            = "audio.volume_step"
	keyBrightnessStep        = "brightness.step"
	keyBlurStrength          = "theme.blur_strength"
)

type Config struct {
	DarkMode              bool
	DoNotDisturb          bool
	InputMode             string // "auto", "tablet", "desktop"
	IdleLockTimeout       int    // seconds; 0 = disabled
	IdleDisplayOffTimeout int    // additional seconds
	IdleSuspendTimeout    int    // additional seconds
	LockMaxAttempts       int
	LockoutDuration       int
	LockShowClock         bool
	LockShowUser          bool
	LidCloseAction        string // "suspend", "lock", "ignore"
	PowerButtonAction     string // "shutdown", "lock", "ignore", "session-menu"

	// New settings
	BarPosition          string  // "top", "bottom"
	BarShowBatteryPct    bool
	ClockFormat          string  // "12h", "24h"
	NotificationTimeout  int     // milliseconds
	NotificationPosition string  // "top-right", "top-left", "bottom-right", "bottom-left"
	VolumeStep           float64 // 0.01 to 0.1
	BrightnessStep       float64 // 0.01 to 0.1
	BlurStrength         int     // 0 to 100
}

func DefaultConfig() Config {
	return Config{
		DarkMode:              true,
		DoNotDisturb:          false,
		InputMode:             "auto",
		IdleLockTimeout:       300,
		IdleDisplayOffTimeout: 30,
		IdleSuspendTimeout:    0,
		LockMaxAttempts:       3,
		LockoutDuration:       30,
		LockShowClock:         true,
		LockShowUser:          true,
		LidCloseAction:        "suspend",
		PowerButtonAction:     "shutdown",

		BarPosition:          "top",
		BarShowBatteryPct:    true,
		ClockFormat:          "24h",
		NotificationTimeout:  5000,
		NotificationPosition: "top-right",
		VolumeStep:           0.05,
		BrightnessStep:       0.05,
		BlurStrength:         20,
	}
}

func Load() (Config, error) {
	d := DefaultConfig()
	return Config{
		DarkMode:              store.LookupOr(keyDarkMode, d.DarkMode),
		DoNotDisturb:          store.LookupOr(keyDoNotDisturb, d.DoNotDisturb),
		InputMode:             store.LookupOr(keyInputMode, d.InputMode),
		IdleLockTimeout:       store.LookupOr(keyIdleLockTimeout, d.IdleLockTimeout),
		IdleDisplayOffTimeout: store.LookupOr(keyIdleDisplayOffTimeout, d.IdleDisplayOffTimeout),
		IdleSuspendTimeout:    store.LookupOr(keyIdleSuspendTimeout, d.IdleSuspendTimeout),
		LockMaxAttempts:       store.LookupOr(keyLockMaxAttempts, d.LockMaxAttempts),
		LockoutDuration:       store.LookupOr(keyLockoutDuration, d.LockoutDuration),
		LockShowClock:         store.LookupOr(keyLockShowClock, d.LockShowClock),
		LockShowUser:          store.LookupOr(keyLockShowUser, d.LockShowUser),
		LidCloseAction:        store.LookupOr(keyLidCloseAction, d.LidCloseAction),
		PowerButtonAction:     store.LookupOr(keyPowerButtonAction, d.PowerButtonAction),

		BarPosition:          store.LookupOr(keyBarPosition, d.BarPosition),
		BarShowBatteryPct:    store.LookupOr(keyBarShowBatteryPct, d.BarShowBatteryPct),
		ClockFormat:          store.LookupOr(keyClockFormat, d.ClockFormat),
		NotificationTimeout:  store.LookupOr(keyNotificationTimeout, d.NotificationTimeout),
		NotificationPosition: store.LookupOr(keyNotificationPosition, d.NotificationPosition),
		VolumeStep:           store.LookupOr(keyVolumeStep, d.VolumeStep),
		BrightnessStep:       store.LookupOr(keyBrightnessStep, d.BrightnessStep),
		BlurStrength:         store.LookupOr(keyBlurStrength, d.BlurStrength),
	}, nil
}

func Save(cfg Config) error {
	return store.SetMany(map[string]any{
		keyDarkMode:              cfg.DarkMode,
		keyDoNotDisturb:          cfg.DoNotDisturb,
		keyInputMode:             cfg.InputMode,
		keyIdleLockTimeout:       cfg.IdleLockTimeout,
		keyIdleDisplayOffTimeout: cfg.IdleDisplayOffTimeout,
		keyIdleSuspendTimeout:    cfg.IdleSuspendTimeout,
		keyLockMaxAttempts:       cfg.LockMaxAttempts,
		keyLockoutDuration:       cfg.LockoutDuration,
		keyLockShowClock:         cfg.LockShowClock,
		keyLockShowUser:          cfg.LockShowUser,
		keyLidCloseAction:        cfg.LidCloseAction,
		keyPowerButtonAction:     cfg.PowerButtonAction,

		keyBarPosition:          cfg.BarPosition,
		keyBarShowBatteryPct:    cfg.BarShowBatteryPct,
		keyClockFormat:          cfg.ClockFormat,
		keyNotificationTimeout:  cfg.NotificationTimeout,
		keyNotificationPosition: cfg.NotificationPosition,
		keyVolumeStep:           cfg.VolumeStep,
		keyBrightnessStep:       cfg.BrightnessStep,
		keyBlurStrength:         cfg.BlurStrength,
	})
}
