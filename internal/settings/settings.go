package settings

import (
	"github.com/sonroyaalmerol/snry-shell/internal/store"
)

const (
	keyDarkMode           = "dark_mode"
	keyDoNotDisturb       = "do_not_disturb"
	keyInputMode          = "input_mode"
	keyIdleLockTimeout    = "idle_lock_timeout"        // seconds; 0 = disabled
	keyIdleDisplayOffTimeout = "idle_displayoff_timeout" // seconds after lock; 0 = disabled
	keyIdleSuspendTimeout = "idle_suspend_timeout"     // seconds after lock; 0 = disabled
	keyLockMaxAttempts    = "lock_max_attempts"        // max password attempts before lockout
	keyLockoutDuration    = "lockout_duration"         // seconds to lock out after max attempts
	keyLockShowClock      = "lock_show_clock"          // show clock on lockscreen
	keyLockShowUser          = "lock_show_user"           // show username on lockscreen
	keyLidCloseAction        = "lid_close_action"         // "suspend", "lock", "ignore"
	keyPowerButtonAction     = "power_button_action"      // "shutdown", "lock", "ignore", "session-menu"
	keyThemeWallpaper        = "theme.wallpaper"
	)

	type Config struct {
	DarkMode              bool
	DoNotDisturb          bool
	InputMode             string // "auto", "tablet", "desktop"
	IdleLockTimeout       int    // seconds before locking; 0 = disabled
	IdleDisplayOffTimeout int    // additional seconds after lock before display off; 0 = disabled
	IdleSuspendTimeout    int    // additional seconds after lock before suspend; 0 = disabled
	LockMaxAttempts       int    // max password attempts before lockout
	LockoutDuration       int    // seconds to lock out after max attempts
	LockShowClock         bool   // show clock on lockscreen
	LockShowUser          bool   // show username on lockscreen
	LidCloseAction        string // "suspend", "lock", "ignore"
	PowerButtonAction     string // "shutdown", "lock", "ignore", "session-menu"
	}

	func DefaultConfig() Config {
	return Config{
		DarkMode:              true,
		DoNotDisturb:          false,
		InputMode:             "auto",
		IdleLockTimeout:       300,
		IdleDisplayOffTimeout: 30, // turn display off 30s after lock by default
		IdleSuspendTimeout:    0,
		LockMaxAttempts:       3,
		LockoutDuration:       30,
		LockShowClock:         true,
		LockShowUser:          true,
		LidCloseAction:        "suspend",
		PowerButtonAction:     "shutdown",
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
	})
	}
