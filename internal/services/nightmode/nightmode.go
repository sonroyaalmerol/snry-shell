package nightmode

import (
	"log"
	"os/exec"
	"sync"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
)

// Runner starts a subprocess.
type Runner interface {
	Start(args ...string) error
}

// Killer kills a named process.
type Killer interface {
	Kill(name string) error
}

type execRunner struct{}

func (e execRunner) Start(args ...string) error {
	return exec.Command(args[0], args[1:]...).Start()
}

type execKiller struct{}

func (e execKiller) Kill(name string) error {
	return exec.Command("pkill", name).Run()
}

// NewRunner returns a Runner backed by the real OS.
func NewRunner() Runner { return execRunner{} }

// NewKiller returns a Killer backed by the real OS.
func NewKiller() Killer { return execKiller{} }

// Service toggles hyprsunset on/off and publishes the enabled state.
type Service struct {
	mu      sync.Mutex
	enabled bool
	runner  Runner
	killer  Killer
	bus     *bus.Bus
	temp    string
}

func New(runner Runner, killer Killer, b *bus.Bus) *Service {
	return &Service{runner: runner, killer: killer, bus: b, temp: "4500"}
}

// Toggle flips the night mode state. Starts or kills hyprsunset accordingly.
func (s *Service) Toggle() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = !s.enabled
	if s.enabled {
		if err := s.runner.Start("hyprsunset", "-t", s.temp); err != nil {
			log.Printf("nightmode start: %v", err)
		}
	} else {
		if err := s.killer.Kill("hyprsunset"); err != nil {
			log.Printf("nightmode stop: %v", err)
		}
	}
	s.bus.Publish(bus.TopicNightMode, s.enabled)
}

// Enabled returns the current state.
func (s *Service) Enabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled
}

// SetTemperature changes the color temperature (e.g. "4500").
func (s *Service) SetTemperature(temp string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.temp = temp
}
