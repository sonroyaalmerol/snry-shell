// Package darkmode monitors and manages the system dark mode preference.
// It uses the xdg-desktop-portal Settings interface when available,
// falling back to gsettings for GNOME/GTK-based environments.
package darkmode

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
)

// Service monitors the system dark mode preference and publishes changes.
type Service struct {
	bus      *bus.Bus
	mu       sync.RWMutex
	isDark   bool
	override bool // Whether shell setting overrides system
}

// New creates a new dark mode service.
func New(b *bus.Bus, cfg settings.Config) *Service {
	return &Service{
		bus:      b,
		isDark:   cfg.DarkMode,
		override: true, // Start with shell setting as override
	}
}

// Run starts monitoring the system dark mode preference.
// It polls for changes since there's no universal signal across all desktops.
func (s *Service) Run(ctx context.Context) error {
	// Initial detection
	s.detect()

	// Subscribe to settings changes from control panel
	s.bus.Subscribe(bus.TopicSettingsChanged, func(e bus.Event) {
		if cfg, ok := e.Data.(settings.Config); ok {
			s.mu.Lock()
			s.isDark = cfg.DarkMode
			s.override = true
			s.mu.Unlock()
			s.publish()
		}
	})

	// Poll for system changes (every 5 seconds)
	// This is a simple approach that works across different desktops
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.detect()
		}
	}
}

// detect checks the current system dark mode preference.
func (s *Service) detect() {
	// Check if we have an override from shell settings
	s.mu.RLock()
	override := s.override
	s.mu.RUnlock()

	if override {
		// Use the shell setting, don't detect system
		return
	}

	// Try xdg-desktop-portal first (most modern)
	if dark, err := s.detectPortal(); err == nil {
		s.update(dark)
		return
	}

	// Fall back to gsettings (GNOME/GTK)
	if dark, err := s.detectGSettings(); err == nil {
		s.update(dark)
		return
	}

	// Default to dark mode as fallback
	s.update(true)
}

// detectPortal tries to read from xdg-desktop-portal.
func (s *Service) detectPortal() (bool, error) {
	// Use dbus-send to query the portal
	// Color scheme: 0 = no preference, 1 = prefer dark, 2 = prefer light
	out, err := exec.Command("dbus-send", "--print-reply", "--dest=org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop", "org.freedesktop.portal.Settings.Read",
		"string:org.freedesktop.appearance", "string:color-scheme").Output()
	if err != nil {
		return false, err
	}

	// Parse output looking for "uint32 1" for dark mode
	if strings.Contains(string(out), "uint32 1") {
		return true, nil
	}
	return false, nil
}

// detectGSettings tries to read from gsettings (GNOME/GTK).
func (s *Service) detectGSettings() (bool, error) {
	out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "color-scheme").Output()
	if err != nil {
		return false, err
	}

	scheme := strings.TrimSpace(string(out))
	scheme = strings.Trim(scheme, "'")

	return scheme == "prefer-dark", nil
}

// update updates the dark mode state and publishes if changed.
func (s *Service) update(dark bool) {
	s.mu.Lock()
	if s.isDark == dark {
		s.mu.Unlock()
		return
	}
	s.isDark = dark
	s.mu.Unlock()
	s.publish()
}

// publish publishes the current dark mode state.
func (s *Service) publish() {
	s.mu.RLock()
	dark := s.isDark
	s.mu.RUnlock()

	s.bus.Publish(bus.TopicDarkMode, dark)
}

// IsDark returns the current dark mode state.
func (s *Service) IsDark() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isDark
}

// UpdateConfig updates the service state from new settings
func (s *Service) UpdateConfig(cfg settings.Config) {
	s.mu.Lock()
	s.isDark = cfg.DarkMode
	s.override = true
	s.mu.Unlock()
	s.publish()
}

// SetOverride sets whether to use shell settings override.
func (s *Service) SetOverride(override bool) {
	s.mu.Lock()
	s.override = override
	s.mu.Unlock()
	if !override {
		s.detect()
	}
}
