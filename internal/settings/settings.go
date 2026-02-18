// Package settings handles loading and saving user configuration.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds all user-configurable shell preferences.
type Config struct {
	DarkMode     bool    `json:"dark_mode"`
	FontScale    float64 `json:"font_scale"`
	BarPosition  string  `json:"bar_position"`
	DoNotDisturb bool    `json:"do_not_disturb"`
	WallpaperDir string  `json:"wallpaper_dir"`
}

// DefaultConfig returns sensible out-of-box defaults.
func DefaultConfig() Config {
	return Config{
		DarkMode:     true,
		FontScale:    1.0,
		BarPosition:  "top",
		DoNotDisturb: false,
		WallpaperDir: filepath.Join(os.Getenv("HOME"), "Pictures", "Wallpapers"),
	}
}

// Load reads the config file; returns defaults if the file does not exist.
func Load() (Config, error) {
	data, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return Config{}, err
	}
	cfg := DefaultConfig() // start from defaults so missing keys keep their default
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Save writes the config to disk, creating parent directories as needed.
func Save(cfg Config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func configPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "snry-shell", "settings.json")
}
