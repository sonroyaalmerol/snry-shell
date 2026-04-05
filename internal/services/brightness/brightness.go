package brightness

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/runner"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

type Service struct {
	runner runner.Runner
	bus    *bus.Bus
}

func New(r runner.Runner, b *bus.Bus) *Service {
	return &Service{runner: r, bus: b}
}

func NewWithDefaults(b *bus.Bus) *Service {
	return New(runner.New(), b)
}

func (s *Service) Run(ctx context.Context) error {
	return runner.PollLoop(ctx, time.Second, s.poll)
}

func (s *Service) poll() {
	bs, err := s.query()
	if err != nil {
		return
	}
	s.bus.Publish(bus.TopicBrightness, bs)
}

// ddcutil output: "VCP code 0x10 (Brightness): current value =    20, max value =   100"
var ddcutilRe = regexp.MustCompile(`current value\s*=\s*(\d+),\s*max value\s*=\s*(\d+)`)

func (s *Service) query() (state.BrightnessState, error) {
	out, err := s.runner.Output("ddcutil", "getvcp", "10")
	if err != nil {
		return state.BrightnessState{}, fmt.Errorf("ddcutil getvcp: %w", err)
	}
	m := ddcutilRe.FindStringSubmatch(string(out))
	if m == nil {
		return state.BrightnessState{}, fmt.Errorf("ddcutil: could not parse output: %q", strings.TrimSpace(string(out)))
	}
	cur, _ := strconv.Atoi(m[1])
	mx, _ := strconv.Atoi(m[2])
	return state.BrightnessState{Current: cur, Max: mx}, nil
}

// SetBrightness sets the monitor brightness. v is 0.0–1.0.
func (s *Service) SetBrightness(v float64) error {
	val := int(v * 100)
	if val < 0 {
		val = 0
	}
	if val > 100 {
		val = 100
	}
	return s.runner.Run("ddcutil", "setvcp", "10", strconv.Itoa(val))
}
