package darkmode

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
)

// Service monitors the system dark mode preference and publishes changes.
type Service struct {
	bus      *bus.Bus
	mu       xsync.RBMutex
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

// NewWithDefaults creates a new dark mode service with default configuration.
func NewWithDefaults(b *bus.Bus) *Service {
	cfg := settings.DefaultConfig()
	if loaded, err := settings.Load(); err == nil {
		cfg = loaded
	}
	return New(b, cfg)
}

// Run starts monitoring the system dark mode preference.
// It polls for changes since there's no universal signal across all desktops.
func (s *Service) Run(ctx context.Context) error {
	// Publish initial state (from settings override)
	s.publish()

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
	t := s.mu.RLock()
	override := s.override
	s.mu.RUnlock(t)

	if override {
		return
	}

	if dark, err := s.detectPortal(); err == nil {
		s.update(dark)
		return
	}

	if dark, err := s.detectGSettings(); err == nil {
		s.update(dark)
		return
	}

	s.update(true)
}

func (s *Service) detectPortal() (bool, error) {
	out, err := exec.Command("dbus-send", "--print-reply", "--dest=org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop", "org.freedesktop.portal.Settings.Read",
		"string:org.freedesktop.appearance", "string:color-scheme").Output()
	if err != nil {
		return false, err
	}
	if strings.Contains(string(out), "uint32 1") {
		return true, nil
	}
	return false, nil
}

func (s *Service) detectGSettings() (bool, error) {
	out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "color-scheme").Output()
	if err != nil {
		return false, err
	}

	scheme := strings.TrimSpace(string(out))
	scheme = strings.Trim(scheme, "'")

	return scheme == "prefer-dark", nil
}

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

func (s *Service) publish() {
	t := s.mu.RLock()
	dark := s.isDark
	s.mu.RUnlock(t)

	s.bus.Publish(bus.TopicDarkMode, dark)
}

// IsDark returns the current dark mode state.
func (s *Service) IsDark() bool {
	t := s.mu.RLock()
	defer s.mu.RUnlock(t)
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
