// Package theme provides wallpaper monitoring and dynamic theme generation.
package theme

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
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
	// Load initial settings
	if cfg, err := settings.Load(); err == nil {
		m.generator.SetBlurStrength(cfg.BlurStrength)
	}

	// Subscribe to settings changes
	m.bus.Subscribe(bus.TopicSettingsChanged, func(e bus.Event) {
		if cfg, ok := e.Data.(settings.Config); ok {
			m.generator.SetBlurStrength(cfg.BlurStrength)
			m.bus.Publish(bus.TopicThemeChanged, bus.Event{Data: m.current})
		}
	})

	// Restore last wallpaper from persistent store and regenerate theme
	if lastWallpaper := store.LookupOr(storeKeyWallpaper, ""); lastWallpaper != "" {
		m.current = lastWallpaper
		// Apply to desktop
		if cfg, err := settings.Load(); err == nil {
			if err := m.applyWallpaper(lastWallpaper, cfg.WallpaperDaemon); err != nil {
				log.Printf("[THEME] failed to apply initial wallpaper: %v", err)
			}
		}
		// Generate theme
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
			m.ensureDaemonRunning()
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
	
	// Apply wallpaper using the configured daemon
	if cfg, err := settings.Load(); err == nil {
		if err := m.applyWallpaper(path, cfg.WallpaperDaemon); err != nil {
			log.Printf("[THEME] Failed to apply wallpaper via daemon: %v", err)
		}
	}

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

func (m *Monitor) ensureDaemonRunning() {
	cfg, err := settings.Load()
	if err != nil {
		return
	}

	daemon := cfg.WallpaperDaemon
	if daemon == "auto" || daemon == "" {
		return
	}

	exe := daemon
	if daemon == "swww" {
		exe = "swww-daemon"
	}

	if !isProcessRunning(exe) {
		log.Printf("[THEME] watchdog: %s is not running, restarting...", daemon)
		if m.current != "" {
			if err := m.startDaemon(m.current, daemon); err != nil {
				log.Printf("[THEME] watchdog: failed to restart %s: %v", daemon, err)
			}
		}
	}
}

func (m *Monitor) startDaemon(path, daemon string) error {
	switch daemon {
	case "swww":
		// swww-daemon needs to be running before swww img
		go exec.Command("swww-daemon").Run()
		// Give it a moment to start
		time.Sleep(500 * time.Millisecond)
		return exec.Command("swww", "img", path).Run()
	case "hyprpaper":
		// Try to start the process
		err := exec.Command("hyprpaper").Start()
		if err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)
		exec.Command("hyprctl", "hyprpaper", "preload", path).Run()
		return exec.Command("hyprctl", "hyprpaper", "wallpaper", ", "+path).Run()
	case "swaybg":
		return exec.Command("swaybg", "-i", path, "-m", "fill").Start()
	case "wbg":
		return exec.Command("wbg", path).Start()
	}
	return nil
}

func (m *Monitor) applyWallpaper(path, daemon string) error {
	if daemon == "auto" {
		// Detect which daemon is running and use it
		if isProcessRunning("swww-daemon") {
			daemon = "swww"
		} else if isProcessRunning("hyprpaper") {
			daemon = "hyprpaper"
		} else if isProcessRunning("swaybg") {
			daemon = "swaybg"
		} else if isProcessRunning("wbg") {
			daemon = "wbg"
		} else {
			return fmt.Errorf("no supported wallpaper daemon detected")
		}
	}

	switch daemon {
	case "swww":
		if !isProcessRunning("swww-daemon") {
			go exec.Command("swww-daemon").Run()
			time.Sleep(500 * time.Millisecond)
		}
		return exec.Command("swww", "img", path).Run()
	case "hyprpaper":
		if !isProcessRunning("hyprpaper") {
			go exec.Command("hyprpaper").Start()
			time.Sleep(500 * time.Millisecond)
		}
		exec.Command("hyprctl", "hyprpaper", "preload", path).Run()
		return exec.Command("hyprctl", "hyprpaper", "wallpaper", ", "+path).Run()
	case "swaybg":
		exec.Command("pkill", "swaybg").Run()
		return exec.Command("swaybg", "-i", path, "-m", "fill").Start()
	case "wbg":
		exec.Command("pkill", "wbg").Run()
		return exec.Command("wbg", path).Start()
	}

	return fmt.Errorf("unsupported daemon: %s", daemon)
}

func isProcessRunning(name string) bool {
	err := exec.Command("pgrep", "-x", name).Run()
	return err == nil
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
