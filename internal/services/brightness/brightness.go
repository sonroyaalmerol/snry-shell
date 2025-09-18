package brightness

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// Runner abstracts running subprocesses for testability.
type Runner interface {
	Output(args ...string) ([]byte, error)
	Run(args ...string) error
}

type execRunner struct{}

func (e execRunner) Output(args ...string) ([]byte, error) {
	return exec.Command(args[0], args[1:]...).Output()
}

func (e execRunner) Run(args ...string) error {
	return exec.Command(args[0], args[1:]...).Run()
}

// NewRunner returns a Runner backed by the real OS.
func NewRunner() Runner { return execRunner{} }

// Service polls brightnessctl and publishes brightness state.
type Service struct {
	runner   Runner
	bus      *bus.Bus
	interval time.Duration
}

func New(runner Runner, b *bus.Bus) *Service {
	return &Service{runner: runner, bus: b, interval: time.Second}
}

func (s *Service) Run(ctx context.Context) error {
	s.poll()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.poll()
		}
	}
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
