package settings

import (
	"github.com/sonroyaalmerol/snry-shell/internal/store"
)

const (
	keyDarkMode            = "dark_mode"
	keyDoNotDisturb        = "do_not_disturb"
	keyInputMode           = "input_mode"
	keyIdleLockTimeout     = "idle_lock_timeout"    // seconds; 0 = disabled
	keyIdleSuspendTimeout  = "idle_suspend_timeout" // seconds after lock; 0 = disabled
)

type Config struct {
	DarkMode           bool
	DoNotDisturb       bool
	InputMode          string // "auto", "tablet", "desktop"
	IdleLockTimeout    int    // seconds before locking; 0 = disabled
	IdleSuspendTimeout int    // additional seconds after lock before suspend; 0 = disabled
}

func DefaultConfig() Config {
	return Config{
		DarkMode:           true,
		DoNotDisturb:       false,
		InputMode:          "auto",
		IdleLockTimeout:    300,
		IdleSuspendTimeout: 0,
	}
}

func Load() (Config, error) {
	d := DefaultConfig()
	return Config{
		DarkMode:           store.LookupOr(keyDarkMode, d.DarkMode),
		DoNotDisturb:       store.LookupOr(keyDoNotDisturb, d.DoNotDisturb),
		InputMode:          store.LookupOr(keyInputMode, d.InputMode),
		IdleLockTimeout:    store.LookupOr(keyIdleLockTimeout, d.IdleLockTimeout),
		IdleSuspendTimeout: store.LookupOr(keyIdleSuspendTimeout, d.IdleSuspendTimeout),
	}, nil
}

func Save(cfg Config) error {
	return store.SetMany(map[string]any{
		keyDarkMode:           cfg.DarkMode,
		keyDoNotDisturb:       cfg.DoNotDisturb,
		keyInputMode:          cfg.InputMode,
		keyIdleLockTimeout:    cfg.IdleLockTimeout,
		keyIdleSuspendTimeout: cfg.IdleSuspendTimeout,
	})
}
