package clipboard

import (
	"bufio"
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
	return runner.PollLoop(ctx, 2*time.Second, s.poll)
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
