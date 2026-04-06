package settings

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/sonroyaalmerol/snry-shell/internal/fileutil"
)

type Config struct {
	DarkMode     bool    `json:"dark_mode"`
	FontScale    float64 `json:"font_scale"`
	BarPosition  string  `json:"bar_position"`
	DoNotDisturb bool    `json:"do_not_disturb"`
	InputMode     string  `json:"input_mode"` // "auto", "tablet", "desktop"

	GestureSensitivity      float64 `json:"gesture_sensitivity"`       // default 1.0
	GestureLongPressDelay   int     `json:"gesture_long_press_delay"`  // default 400ms
	GestureWorkspaceFingers int     `json:"gesture_workspace_fingers"` // default 3
}

func DefaultConfig() Config {
	return Config{
		DarkMode:     true,
		FontScale:    1.0,
		BarPosition:  "top",
		DoNotDisturb: false,
		InputMode:     "auto",
		GestureSensitivity:      1.0,
		GestureLongPressDelay:   400,
		GestureWorkspaceFingers: 3,
	}
}

func Load() (Config, error) {
	data, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return Config{}, err
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(cfg Config) error {
	return fileutil.SaveJSON(configPath(), cfg)
}

func configPath() string {
	return filepath.Join(fileutil.ConfigDir(), "settings.json")
}
