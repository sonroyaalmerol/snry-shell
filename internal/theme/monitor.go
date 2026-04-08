// Package theme provides wallpaper monitoring and dynamic theme generation.
package theme

import (
	"context"
	"fmt"
	"log"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
	"github.com/sonroyaalmerol/snry-shell/internal/store"
)

// Monitor watches for wallpaper/settings changes and regenerates themes.
type Monitor struct {
	bus       *bus.Bus
	generator *Generator
	// current is the path of the processed wallpaper (what surfaces display).
	current string
	// source is the original user-selected path.
	source string
}

// NewMonitor creates a new wallpaper monitor.
func NewMonitor(b *bus.Bus) *Monitor {
	return &Monitor{
		bus:       b,
		generator: New(),
	}
}

// Run starts the monitor. It restores and re-processes the last wallpaper,
// then stays alive to re-process whenever settings change.
func (m *Monitor) Run(ctx context.Context) {
	cfg, _ := settings.Load()
	m.generator.SetBlurStrength(cfg.BlurStrength)

	// Re-process and restore last wallpaper from saved source.
	if cfg.WallpaperSource != "" {
		m.source = cfg.WallpaperSource
		if err := m.reprocess(cfg); err != nil {
			log.Printf("[THEME] restore wallpaper: %v", err)
		}
	} else if last := GetLastWallpaper(); last != "" {
		// Fallback: a processed path exists but source is not yet recorded
		// (e.g. store from before source tracking was added).
		m.current = last
		if err := m.generator.SetWallpaper(last); err != nil {
			log.Printf("[THEME] restore legacy wallpaper: %v", err)
			store.Delete(storeKeyWallpaper)
		} else {
			m.bus.Publish(bus.TopicThemeChanged, bus.Event{Data: last})
		}
	}

	// Re-process whenever processing-related settings change.
	m.bus.Subscribe(bus.TopicSettingsChanged, func(e bus.Event) {
		newCfg, ok := e.Data.(settings.Config)
		if !ok {
			return
		}
		m.generator.SetBlurStrength(newCfg.BlurStrength)

		if m.source == "" {
			// Blur-strength-only change still needs a theme CSS refresh.
			if m.current != "" {
				m.bus.Publish(bus.TopicThemeChanged, bus.Event{Data: m.current})
			}
			return
		}

		// Only re-process if the processing parameters changed.
		oldCfg := cfg
		cfg = newCfg
		if newCfg.WallpaperBlur != oldCfg.WallpaperBlur ||
			newCfg.WallpaperBrightness != oldCfg.WallpaperBrightness ||
			newCfg.WallpaperGrayscale != oldCfg.WallpaperGrayscale {
			go func() {
				if err := m.reprocess(newCfg); err != nil {
					log.Printf("[THEME] reprocess on settings change: %v", err)
				}
			}()
		} else {
			// CSS-only change (blur strength, etc.) — just republish.
			m.bus.Publish(bus.TopicThemeChanged, bus.Event{Data: m.current})
		}
	})

	<-ctx.Done()
}

// ForceUpdate forces a theme CSS regeneration from the current processed wallpaper.
func (m *Monitor) ForceUpdate() error {
	if m.current == "" {
		return nil
	}
	if err := m.generator.Generate(); err != nil {
		return fmt.Errorf("force update: %w", err)
	}
	m.bus.Publish(bus.TopicThemeChanged, bus.Event{Data: m.current})
	return nil
}

// SetWallpaper copies sourcePath to the fixed processed location (applying any
// active adjustments), regenerates theme colours, and notifies subscribers.
func (m *Monitor) SetWallpaper(sourcePath string) error {
	cfg, err := settings.Load()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	m.source = sourcePath

	// Persist the source path so it survives restarts.
	cfg.WallpaperSource = sourcePath
	if err := settings.Save(cfg); err != nil {
		log.Printf("[THEME] save wallpaper source: %v", err)
	}

	return m.reprocess(cfg)
}

// reprocess applies processing settings to m.source, updates m.current,
// regenerates theme CSS, and publishes TopicThemeChanged.
// Must be safe to call from any goroutine.
func (m *Monitor) reprocess(cfg settings.Config) error {
	if m.source == "" {
		return nil
	}

	pcfg := ProcessConfig{
		Blur:       cfg.WallpaperBlur,
		Brightness: cfg.WallpaperBrightness,
		Grayscale:  cfg.WallpaperGrayscale,
	}

	processed, err := ProcessWallpaper(m.source, pcfg)
	if err != nil {
		return fmt.Errorf("process wallpaper: %w", err)
	}

	m.current = processed

	if err := store.Set(storeKeyWallpaper, processed); err != nil {
		log.Printf("[THEME] save processed path: %v", err)
	}

	if err := m.generator.SetWallpaper(processed); err != nil {
		return fmt.Errorf("generate theme: %w", err)
	}

	m.bus.Publish(bus.TopicThemeChanged, bus.Event{Data: processed})
	return nil
}
