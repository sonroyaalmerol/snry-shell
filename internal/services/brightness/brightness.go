package brightness

import (
	"context"
	"fmt"
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

func (s *Service) query() (state.BrightnessState, error) {
	current, err := s.runner.Output("brightnessctl", "get")
	if err != nil {
		return state.BrightnessState{}, fmt.Errorf("brightnessctl get: %w", err)
	}
	max, err := s.runner.Output("brightnessctl", "max")
	if err != nil {
		return state.BrightnessState{}, fmt.Errorf("brightnessctl max: %w", err)
	}
	return ParseBrightnessctl(string(current), string(max))
}

// ParseBrightnessctl parses the output of `brightnessctl get` and `brightnessctl max`.
func ParseBrightnessctl(current, max string) (state.BrightnessState, error) {
	cur, err := strconv.Atoi(strings.TrimSpace(current))
	if err != nil {
		return state.BrightnessState{}, fmt.Errorf("parse current brightness: %w", err)
	}
	mx, err := strconv.Atoi(strings.TrimSpace(max))
	if err != nil {
		return state.BrightnessState{}, fmt.Errorf("parse max brightness: %w", err)
	}
	return state.BrightnessState{Current: cur, Max: mx}, nil
}

// SetBrightness sets the screen brightness. v is 0.0–1.0.
func (s *Service) SetBrightness(v float64) error {
	pct := fmt.Sprintf("%.0f%%", v*100)
	return s.runner.Run("brightnessctl", "set", pct+"@")
}
