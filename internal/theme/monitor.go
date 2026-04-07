// Package theme provides wallpaper monitoring and dynamic theme generation.
package theme

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/store"
)

// Monitor watches for wallpaper changes and regenerates themes
type Monitor struct {
	bus       *bus.Bus
	generator *Generator
	current   string
	stop      chan struct{}
}

// NewMonitor creates a new wallpaper monitor
func NewMonitor(b *bus.Bus) *Monitor {
	return &Monitor{
		bus:       b,
		generator: New(),
		stop:      make(chan struct{}),
	}
}

// Run starts monitoring for wallpaper changes
func (m *Monitor) Run(ctx context.Context) {
	// Restore last wallpaper from persistent store and regenerate theme
	if lastWallpaper := store.LookupOr(storeKeyWallpaper, ""); lastWallpaper != "" {
		m.current = lastWallpaper
		if err := m.generator.SetWallpaper(lastWallpaper); err != nil {
			// If failed (file might be gone), clear it
			store.Delete(storeKeyWallpaper)
		}
	}

	// Initial check for new wallpaper
	m.checkAndUpdate()

	// Poll for changes every 5 seconds
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stop:
			return
		case <-ticker.C:
			m.checkAndUpdate()
		}
	}
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	close(m.stop)
}

// ForceUpdate forces a theme regeneration from current wallpaper
func (m *Monitor) ForceUpdate() error {
	return m.checkAndUpdate()
}

// SetWallpaper manually sets a wallpaper path and regenerates theme
func (m *Monitor) SetWallpaper(path string) error {
	m.current = path
	if err := m.generator.SetWallpaper(path); err != nil {
		return fmt.Errorf("set wallpaper: %w", err)
	}
	// Save to persistent store
	if err := store.Set(storeKeyWallpaper, path); err != nil {
		return fmt.Errorf("save wallpaper path: %w", err)
	}
	m.bus.Publish(bus.TopicThemeChanged, bus.Event{Data: path})
	return nil
}

func (m *Monitor) checkAndUpdate() error {
	wallpaper, err := getCurrentWallpaper()
	if err != nil {
		return err
	}

	if wallpaper == "" || wallpaper == m.current {
		return nil
	}

	// Resolve to absolute path
	if !filepath.IsAbs(wallpaper) {
		if abs, err := filepath.Abs(wallpaper); err == nil {
			wallpaper = abs
		}
	}

	m.current = wallpaper

	if err := m.generator.SetWallpaper(wallpaper); err != nil {
		return fmt.Errorf("generate theme: %w", err)
	}

	// Save to persistent store
	if err := store.Set(storeKeyWallpaper, wallpaper); err != nil {
		return fmt.Errorf("save wallpaper path: %w", err)
	}

	// Notify that theme changed
	m.bus.Publish(bus.TopicThemeChanged, bus.Event{Data: wallpaper})

	return nil
}

// getCurrentWallpaper tries to get the current wallpaper from various sources
func getCurrentWallpaper() (string, error) {
	// Try hyprctl first (Hyprland)
	if path := getHyprlandWallpaper(); path != "" {
		return path, nil
	}

	// Try swww
	if path := getSWWWWallpaper(); path != "" {
		return path, nil
	}

	// Try nitrogen
	if path := getNitrogenWallpaper(); path != "" {
		return path, nil
	}

	// Try feh
	if path := getFehWallpaper(); path != "" {
		return path, nil
	}

	return "", fmt.Errorf("no wallpaper tool detected")
}

func getHyprlandWallpaper() string {
	out, err := exec.Command("hyprctl", "hyprpaper", "listactive").Output()
	if err != nil {
		return ""
	}

	// Parse output like "monitor = HDMI-A-1, /path/to/wallpaper.jpg"
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}

	return ""
}

func getSWWWWallpaper() string {
	out, err := exec.Command("swww", "query").Output()
	if err != nil {
		return ""
	}

	// Parse output like "HDMI-A-1: /path/to/wallpaper.jpg"
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		parts := strings.Split(line, ": ")
		if len(parts) >= 2 {
			return strings.TrimSpace(parts[1])
		}
	}

	return ""
}

var nitrogenBgRegex = regexp.MustCompile(`file=(.+)`)

func getNitrogenWallpaper() string {
	if _, err := exec.Command("nitrogen", "--save").Output(); err != nil {
		// nitrogen not running — fall back to reading its config file directly.
		cfgPath := filepath.Join(os.Getenv("HOME"), ".config", "nitrogen", "bg-saved.cfg")
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			return ""
		}
		if m := nitrogenBgRegex.FindSubmatch(data); len(m) >= 2 {
			return strings.TrimSpace(string(m[1]))
		}
	}
	return ""
}

var fehImgRe = regexp.MustCompile(`\S+\.(jpg|jpeg|png|gif|bmp|webp)`)

func getFehWallpaper() string {
	// feh stores its last command in ~/.fehbg, e.g.:
	//   feh --no-fehbg --bg-fill /path/to/image.jpg
	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".fehbg"))
	if err != nil {
		return ""
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		if m := fehImgRe.FindString(sc.Text()); m != "" {
			return m
		}
	}
	return ""
}
