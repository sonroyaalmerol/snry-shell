package audio

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
	return runner.PollLoop(ctx, 200*time.Millisecond, s.poll)
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
