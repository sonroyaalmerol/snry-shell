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

// hyprpm plugin manifest path.
const hyprpmRoot = ".local/share/hyprpm"

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

// install uses hyprpm to add and build the hyprgrass plugin.
func (s *Service) install(ctx context.Context, home string) (string, error) {
	if err := runCmd(ctx, "hyprpm", "add", "https://github.com/horriblename/hyprgrass"); err != nil {
		return "", fmt.Errorf("hyprpm add: %w", err)
	}
	if err := runCmd(ctx, "hyprpm", "ensure"); err != nil {
		return "", fmt.Errorf("hyprpm ensure: %w", err)
	}
	path := s.findPlugin(home)
	if path == "" {
		return "", fmt.Errorf("hyprpm completed but .so not found")
	}
	return path, nil
}

func runCmd(ctx context.Context, name string, args ...string) error {
	log.Printf("[HYPRGRASS] running: %s %v", name, args)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	return cmd.Run()
}
