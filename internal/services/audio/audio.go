package audio

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

// Runner abstracts running subprocesses so tests can inject a fake.
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

// Service polls wpctl for the default sink volume and publishes updates.
type Service struct {
	runner   Runner
	bus      *bus.Bus
	interval time.Duration
}

func New(runner Runner, b *bus.Bus) *Service {
	return &Service{runner: runner, bus: b, interval: 500 * time.Millisecond}
}

func (s *Service) Run(ctx context.Context) error {
	// Emit once immediately, then poll.
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
	sink, err := s.query()
	if err != nil {
		return
	}
	s.bus.Publish(bus.TopicAudio, sink)
}

func (s *Service) query() (state.AudioSink, error) {
	out, err := s.runner.Output("wpctl", "get-volume", "@DEFAULT_SINK@")
	if err != nil {
		return state.AudioSink{}, err
	}
	return ParseWpctlVolume(string(out))
}

// ParseWpctlVolume is exported for tests.
// Input examples:
//
//	"Volume: 0.75"
//	"Volume: 0.40 [MUTED]"
func ParseWpctlVolume(output string) (state.AudioSink, error) {
	output = strings.TrimSpace(output)
	muted := strings.Contains(output, "[MUTED]")
	fields := strings.Fields(output)
	if len(fields) < 2 {
		return state.AudioSink{}, fmt.Errorf("unexpected wpctl output: %q", output)
	}
	vol, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return state.AudioSink{}, fmt.Errorf("parse volume: %w", err)
	}
	return state.AudioSink{Volume: vol, Muted: muted}, nil
}

// SetVolume sets the default sink volume. v is 0.0–1.0.
func (s *Service) SetVolume(v float64) error {
	pct := fmt.Sprintf("%.0f%%", v*100)
	_, err := s.runner.Output("wpctl", "set-volume", "@DEFAULT_SINK@", pct)
	return err
}

// SetMuted sets the mute state on the default sink.
func (s *Service) SetMuted(muted bool) error {
	flag := "0"
	if muted {
		flag = "1"
	}
	_, err := s.runner.Output("wpctl", "set-mute", "@DEFAULT_SINK@", flag)
	return err
}

// ToggleMute flips the mute state.
func (s *Service) ToggleMute() error {
	_, err := s.runner.Output("wpctl", "set-mute", "@DEFAULT_SINK@", "toggle")
	return err
}
