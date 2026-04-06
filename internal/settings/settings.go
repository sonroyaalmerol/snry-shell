package settings

import (
	"github.com/sonroyaalmerol/snry-shell/internal/store"
)

const (
	keyDarkMode     = "dark_mode"
	keyFontScale    = "font_scale"
	keyBarPosition  = "bar_position"
	keyDoNotDisturb = "do_not_disturb"
	keyInputMode    = "input_mode"
)

type Config struct {
	DarkMode     bool
	FontScale    float64
	BarPosition  string
	DoNotDisturb bool
	InputMode    string // "auto", "tablet", "desktop"
}

func DefaultConfig() Config {
	return Config{
		DarkMode:     true,
		FontScale:    1.0,
		BarPosition:  "top",
		DoNotDisturb: false,
		InputMode:    "auto",
	}
}

func Load() (Config, error) {
	d := DefaultConfig()
	return Config{
		DarkMode:     store.LookupOr(keyDarkMode, d.DarkMode),
		FontScale:    store.LookupOr(keyFontScale, d.FontScale),
		BarPosition:  store.LookupOr(keyBarPosition, d.BarPosition),
		DoNotDisturb: store.LookupOr(keyDoNotDisturb, d.DoNotDisturb),
		InputMode:    store.LookupOr(keyInputMode, d.InputMode),
	}, nil
}

func Save(cfg Config) error {
	return store.SetMany(map[string]any{
		keyDarkMode:     cfg.DarkMode,
		keyFontScale:    cfg.FontScale,
		keyBarPosition:  cfg.BarPosition,
		keyDoNotDisturb: cfg.DoNotDisturb,
		keyInputMode:    cfg.InputMode,
	})
}
