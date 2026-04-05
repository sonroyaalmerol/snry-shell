package clipboard

import (
	"bufio"
	"context"
	"fmt"
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
	s.poll()

	rc, err := s.streamer.Stream("wl-paste", "--watch", "sh", "-c", "echo c")
	if err != nil {
		log.Printf("[clipboard] wl-paste --watch unavailable, falling back to polling: %v", err)
		return runner.PollLoop(ctx, 2*time.Second, s.poll)
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
		// Drain rapid successive triggers.
		time.Sleep(50 * time.Millisecond)
		for sc.Scan() {
		}
		s.poll()
	}
}

func (s *Service) poll() {
	entries, err := s.query()
	if err != nil {
		return
	}
	s.bus.Publish(bus.TopicClipboard, entries)
}

func (s *Service) query() ([]state.ClipboardEntry, error) {
	out, err := s.runner.Output("cliphist", "list")
	if err != nil {
		return nil, fmt.Errorf("cliphist list: %w", err)
	}
	return ParseCliphistList(string(out))
}

// ParseCliphistList parses the output of `cliphist list`.
// Each line is: "<id>\t<preview>"
func ParseCliphistList(output string) ([]state.ClipboardEntry, error) {
	var entries []state.ClipboardEntry
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		id, preview, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(id))
		if err != nil {
			continue
		}
		entries = append(entries, state.ClipboardEntry{ID: n, Preview: preview})
	}
	return entries, scanner.Err()
}
