package brightness

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/ddc"
	"github.com/sonroyaalmerol/snry-shell/internal/services/runner"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const backlightRoot = "/sys/class/backlight"

type Service struct {
	bus            *bus.Bus
	last           state.BrightnessState
	lastErr        string
	brightnessStep float64
}

func New(b *bus.Bus) *Service {
	return &Service{bus: b, brightnessStep: 0.05}
}

func (s *Service) UpdateStep(step float64) {
	s.brightnessStep = step
}

func (s *Service) BrightnessStep() float64 {
	return s.brightnessStep
}

func NewWithDefaults(b *bus.Bus) *Service {
	return New(b)
}

func (s *Service) Run(ctx context.Context) error {
	return runner.PollLoop(ctx, 2*time.Second, s.poll)
}

func (s *Service) poll() {
	// Prefer DDC for external monitors.
	if v, err := ddc.GetVCP(0x10); err == nil {
		s.lastErr = ""
		bs := state.BrightnessState{Current: int(v.Current), Max: int(v.Max)}
		if bs.Current != s.last.Current || bs.Max != s.last.Max {
			s.last = bs
			s.bus.Publish(bus.TopicBrightness, bs)
		}
		return
	}

	// Fall back to the first /sys/class/backlight device.
	dev := backlightDevice()
	if dev == "" {
		errStr := "no DDC or backlight device found"
		if errStr != s.lastErr {
			log.Printf("[brightness] %s", errStr)
			s.lastErr = errStr
		}
		return
	}
	cur, max, err := backlightGet(dev)
	if err != nil {
		errStr := err.Error()
		if errStr != s.lastErr {
			log.Printf("[brightness] backlight: %v", err)
			s.lastErr = errStr
		}
		return
	}
	s.lastErr = ""
	bs := state.BrightnessState{Current: cur, Max: max}
	if bs.Current != s.last.Current || bs.Max != s.last.Max {
		s.last = bs
		s.bus.Publish(bus.TopicBrightness, bs)
	}
}

// SetBrightness sets brightness as a fraction 0.0–1.0 using DDC or backlight.
func (s *Service) SetBrightness(value float64) error {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}

	var err error
	// Try DDC first.
	if val, ddcErr := ddc.GetVCP(0x10); ddcErr == nil {
		raw := min(int(value*float64(val.Max)), int(val.Max))
		err = ddc.SetVCP(0x10, uint16(raw))
	} else {
		// Fall back to backlight sysfs.
		dev := backlightDevice()
		if dev == "" {
			return fmt.Errorf("no DDC or backlight device available")
		}
		_, max, bErr := backlightGet(dev)
		if bErr != nil {
			return bErr
		}
		err = backlightSet(dev, int(value*float64(max)))
	}

	if err == nil {
		go s.poll()
	}
	return err
}

// AdjustBrightness changes brightness by delta (e.g. +0.05 / -0.05), clamped to [0, 1].
func (s *Service) AdjustBrightness(delta float64) error {
	var err error

	if val, ddcErr := ddc.GetVCP(0x10); ddcErr == nil {
		current := int(val.Current)
		max := int(val.Max)
		if max == 0 {
			return fmt.Errorf("DDC max brightness is 0")
		}
		next := float64(current)/float64(max) + delta
		if next < 0 {
			next = 0
		}
		if next > 1 {
			next = 1
		}
		err = ddc.SetVCP(0x10, uint16(next*float64(max)))
	} else {
		dev := backlightDevice()
		if dev == "" {
			return fmt.Errorf("no DDC or backlight device available")
		}
		current, max, bErr := backlightGet(dev)
		if bErr != nil {
			return bErr
		}
		if max == 0 {
			return fmt.Errorf("backlight max is 0")
		}
		next := float64(current)/float64(max) + delta
		if next < 0 {
			next = 0
		}
		if next > 1 {
			next = 1
		}
		err = backlightSet(dev, int(next*float64(max)))
	}

	if err == nil {
		go s.poll()
	}
	return err
}

// ── backlight sysfs helpers ───────────────────────────────────────────────────

// backlightDevice returns the first backlight device name found in sysfs.
func backlightDevice() string {
	entries, err := os.ReadDir(backlightRoot)
	if err != nil {
		return ""
	}
	// Prefer "intel_backlight" or "amdgpu_bl*" over generic "acpi_video*".
	var fallback string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "intel_") || strings.HasPrefix(name, "amdgpu_") ||
			strings.HasPrefix(name, "nvidia_") {
			return name
		}
		if fallback == "" {
			fallback = name
		}
	}
	return fallback
}

func backlightGet(device string) (current, max int, err error) {
	read := func(file string) (int, error) {
		data, err := os.ReadFile(filepath.Join(backlightRoot, device, file))
		if err != nil {
			return 0, err
		}
		return strconv.Atoi(strings.TrimSpace(string(data)))
	}
	cur, err := read("brightness")
	if err != nil {
		return 0, 0, fmt.Errorf("read brightness: %w", err)
	}
	mx, err := read("max_brightness")
	if err != nil {
		return 0, 0, fmt.Errorf("read max_brightness: %w", err)
	}
	return cur, mx, nil
}

func backlightSet(device string, value int) error {
	path := filepath.Join(backlightRoot, device, "brightness")
	return os.WriteFile(path, []byte(strconv.Itoa(value)), 0644)
}
