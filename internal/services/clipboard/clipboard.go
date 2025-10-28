package clipboard

import (
	"bufio"
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
}

type execRunner struct{}

func (e execRunner) Output(args ...string) ([]byte, error) {
	return exec.Command(args[0], args[1:]...).Output()
}

// NewRunner returns a Runner backed by the real OS.
func NewRunner() Runner { return execRunner{} }

// Service polls cliphist for clipboard entries and publishes them.
type Service struct {
	runner   Runner
	bus      *bus.Bus
	interval time.Duration
}

func New(runner Runner, b *bus.Bus) *Service {
	return &Service{runner: runner, bus: b, interval: 2 * time.Second}
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
