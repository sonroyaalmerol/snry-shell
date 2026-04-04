package audiomixer

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
	return runner.PollLoop(ctx, 2*time.Second, s.publish)
}

func (s *Service) publish() {
	apps, err := s.listApps()
	if err != nil {
		return
	}
	s.bus.Publish(bus.TopicAudioMixer, state.AudioMixerState{Apps: apps})
}

func (s *Service) listApps() ([]state.AudioApp, error) {
	out, err := s.runner.Output("pactl", "list", "sink-inputs", "short")
	if err != nil {
		return nil, err
	}

	var apps []state.AudioApp
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		id, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		name := fields[2]
		muted := fields[5] == "MUTED"

		// Volume field format: "45%" or "32768 / 65536"
		vol := 0.5
		if strings.Contains(fields[6], "%") {
			pct := strings.TrimSuffix(fields[6], "%")
			v, _ := strconv.ParseFloat(pct, 64)
			vol = v / 100
		} else if strings.Contains(fields[6], "/") {
			parts := strings.Split(fields[6], "/")
			if len(parts) == 2 {
				cur, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
				max, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				if max > 0 {
					vol = cur / max
				}
			}
		}

		apps = append(apps, state.AudioApp{
			Name:   name,
			ID:     id,
			Volume: vol,
			Muted:  muted,
		})
	}
	return apps, nil
}

func (s *Service) SetAppVolume(id int, vol float64) error {
	pct := fmt.Sprintf("%.0f%%", vol*100)
	_, err := s.runner.Output("pactl", "set-sink-input-volume", strconv.Itoa(id), pct)
	return err
}

func (s *Service) ToggleAppMute(id int) error {
	_, err := s.runner.Output("pactl", "set-sink-input-mute", strconv.Itoa(id), "toggle")
	return err
}
