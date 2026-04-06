package settings

import (
	"github.com/sonroyaalmerol/snry-shell/internal/store"
)

const (
	keyDarkMode     = "dark_mode"
	keyDoNotDisturb = "do_not_disturb"
	keyInputMode    = "input_mode"
)

type Config struct {
	DarkMode     bool
	DoNotDisturb bool
	InputMode    string // "auto", "tablet", "desktop"
}

func DefaultConfig() Config {
	return Config{
		DarkMode:     true,
		DoNotDisturb: false,
		InputMode:    "auto",
	}
}

func Load() (Config, error) {
	d := DefaultConfig()
	return Config{
		DarkMode:     store.LookupOr(keyDarkMode, d.DarkMode),
		DoNotDisturb: store.LookupOr(keyDoNotDisturb, d.DoNotDisturb),
		InputMode:    store.LookupOr(keyInputMode, d.InputMode),
	}, nil
}

func Save(cfg Config) error {
	return store.SetMany(map[string]any{
		keyDarkMode:     cfg.DarkMode,
		keyDoNotDisturb: cfg.DoNotDisturb,
		keyInputMode:    cfg.InputMode,
	})
}
