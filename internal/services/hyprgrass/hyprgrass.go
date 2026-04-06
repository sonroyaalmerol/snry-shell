// Package hyprgrass auto-installs and loads the hyprgrass Hyprland plugin
// for touch gesture support. Since Hyprland holds exclusive session control
// over input devices, touch gestures must be handled by an in-process plugin.
package hyprgrass

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
)

const (
	// hyprpm plugin manifest path.
	hyprpmRoot = ".local/share/hyprpm"
	// hyprpm per-user state file path (may be root-owned).
	hyprpmStatePath = "/var/cache/hyprpm/%s/state.toml"
)

// Service manages hyprgrass plugin installation and loading.
type Service struct {
	querier *hyprland.Querier
}

// New creates the hyprgrass service.
func New(q *hyprland.Querier) *Service {
	return &Service{querier: q}
}

// Run ensures hyprgrass is installed and loaded. Blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	home, _ := os.UserHomeDir()
	pluginPath := s.findPlugin(home)

	if pluginPath == "" {
		log.Printf("[HYPRGRASS] plugin not found, installing via hyprpm...")
		var err error
		pluginPath, err = s.install(ctx, home)
		if err != nil {
			return fmt.Errorf("hyprgrass install: %w", err)
		}
	}

	log.Printf("[HYPRGRASS] loading plugin: %s", pluginPath)
	if err := s.querier.SetKeyword("plugin", pluginPath); err != nil {
		return fmt.Errorf("hyprgrass load: %w", err)
	}
	log.Printf("[HYPRGRASS] plugin loaded successfully")

	s.applyGestures()

	<-ctx.Done()
	return ctx.Err()
}

// findPlugin looks for the hyprgrass .so in known locations.
func (s *Service) findPlugin(home string) string {
	// hyprpm installs to ~/.local/share/hyprpm/<name>/<hash>/libhyprgrass.so
	hyprpmDir := filepath.Join(home, hyprpmRoot)
	if entries, err := os.ReadDir(hyprpmDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			versions, err := os.ReadDir(filepath.Join(hyprpmDir, entry.Name()))
			if err != nil {
				continue
			}
			for _, ver := range versions {
				so := filepath.Join(hyprpmDir, entry.Name(), ver.Name(), "libhyprgrass.so")
				if _, err := os.Stat(so); err == nil {
					return so
				}
			}
		}
	}

	// Check system path.
	if _, err := os.Stat("/usr/lib/libhyprgrass.so"); err == nil {
		return "/usr/lib/libhyprgrass.so"
	}

	return ""
}

// install uses hyprpm to add and enable the hyprgrass plugin.
func (s *Service) install(ctx context.Context, home string) (string, error) {
	// Update headers to match current Hyprland version.
	if err := runCmd(ctx, "hyprpm", "update"); err != nil {
		log.Printf("[HYPRGRASS] hyprpm update failed (non-fatal): %v", err)
	}
	// Add the plugin repository.
	if err := runCmd(ctx, "hyprpm", "add", "https://github.com/horriblename/hyprgrass"); err != nil {
		// Likely "Headers outdated" due to root-owned state.toml.
		home, _ := os.UserHomeDir()
		user := filepath.Base(home)
		statePath := fmt.Sprintf(hyprpmStatePath, user)
		log.Printf("[HYPRGRASS] hyprpm add failed. Try: sudo chown %s %s", os.Getenv("USER"), statePath)
		return "", fmt.Errorf("hyprpm add: %w", err)
	}
	// Enable the plugin so it builds and gets loaded.
	if err := runCmd(ctx, "hyprpm", "enable", "hyprgrass"); err != nil {
		return "", fmt.Errorf("hyprpm enable: %w", err)
	}
	path := s.findPlugin(home)
	if path == "" {
		return "", fmt.Errorf("hyprpm completed but .so not found")
	}
	return path, nil
}

// applyGestures registers common touch gesture bindings via hyprctl keyword bindl.
func (s *Service) applyGestures() {
	for _, b := range gestureBinds {
		if err := s.querier.SetKeyword("bindl", b); err != nil {
			log.Printf("[HYPRGRASS] failed to bind %q: %v", b, err)
		}
	}
}

// gestureBinds are hyprgrass gesture bindings in "bindl" format.
// Syntax: bindl = , <gesture_trigger>, <dispatcher>, <args>
var gestureBinds = []string{
	// 3-finger swipe: workspace navigation
	", swipe:3:l, workspace, -1",
	", swipe:3:r, workspace, +1",
	", swipe:3:u, workspace, e-1",
	", swipe:3:d, workspace, e+1",

	// 4-finger swipe: fullscreen and split
	", swipe:4:u, fullscreen, 1",
	", swipe:4:d, fullscreen, 0",

	// Long press: kill active window
	", longpress:2, killactive",
}

func runCmd(ctx context.Context, name string, args ...string) error {
	log.Printf("[HYPRGRASS] running: %s %v", name, args)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	return cmd.Run()
}
