package brightness

import (
	"context"
	"log"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/ddc"
	"github.com/sonroyaalmerol/snry-shell/internal/services/runner"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

type Service struct {
	bus        *bus.Bus
	last       state.BrightnessState
	lastErr    string // suppresses repeated identical errors
}

func New(b *bus.Bus) *Service {
	return &Service{bus: b}
}

func NewWithDefaults(b *bus.Bus) *Service {
	return New(b)
}

func (s *Service) Run(ctx context.Context) error {
	return runner.PollLoop(ctx, 2*time.Second, s.poll)
}

func (s *Service) poll() {
	v, err := ddc.GetVCP(0x10)
	if err != nil {
		errStr := err.Error()
		if errStr != s.lastErr {
			log.Printf("[brightness] ddc get: %v", err)
			s.lastErr = errStr
		}
		return
	}
	// Reset error suppression on success so new errors get logged once.
	s.lastErr = ""
	bs := state.BrightnessState{Current: int(v.Current), Max: int(v.Max)}
	if bs.Current == s.last.Current && bs.Max == s.last.Max {
		return
	}
	s.last = bs
	s.bus.Publish(bus.TopicBrightness, bs)
}

// SetBrightness sets the monitor brightness. v is 0.0-1.0.
func (s *Service) SetBrightness(v float64) error {
	if s.last.Max == 0 {
		// Read current max first if unknown.
		val, err := ddc.GetVCP(0x10)
		if err != nil {
			return err
		}
		s.last.Max = int(val.Max)
	}
	raw := int(v * float64(s.last.Max))
	if raw < 0 {
		raw = 0
	}
	if raw > s.last.Max {
		raw = s.last.Max
	}
	return ddc.SetVCP(0x10, uint16(raw))
}
