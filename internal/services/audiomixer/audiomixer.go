package audiomixer

import (
	"bufio"
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/runner"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

type Service struct {
	runner   runner.Runner
	streamer runner.StreamReader
	bus      *bus.Bus
}

func New(r runner.Runner, sr runner.StreamReader, b *bus.Bus) *Service {
	return &Service{runner: r, streamer: sr, bus: b}
}

func NewWithDefaults(b *bus.Bus) *Service {
	return New(runner.New(), runner.NewStreamReader(), b)
}

func (s *Service) Run(ctx context.Context) error {
	s.publish()

	rc, err := s.streamer.Stream("pactl", "subscribe")
	if err != nil {
		log.Printf("[audiomixer] pactl subscribe unavailable, falling back to polling: %v", err)
		return runner.PollLoop(ctx, 2*time.Second, s.publish)
	}
	defer rc.Close()

	sc := bufio.NewScanner(rc)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				return err
			}
			return nil
		}
		line := sc.Text()
		if !strings.Contains(line, "sink-input") || !strings.Contains(line, "change") {
			continue
		}
		time.Sleep(50 * time.Millisecond)
		for sc.Scan() {
			next := sc.Text()
			if !strings.Contains(next, "sink-input") || !strings.Contains(next, "change") {
				break
			}
		}
		s.publish()
	}
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

