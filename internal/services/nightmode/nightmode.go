package nightmode

import (
	"log"
	"os/exec"
	"sync/atomic"

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
	cmd := exec.Command(args[0], args[1:]...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
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
	enabled atomic.Bool
	runner  Runner
	killer  Killer
	bus     *bus.Bus
	temp    string
}

func New(runner Runner, killer Killer, b *bus.Bus) *Service {
	return &Service{runner: runner, killer: killer, bus: b, temp: "4500"}
}

func NewWithDefaults(b *bus.Bus) *Service {
	return New(NewRunner(), NewKiller(), b)
}

// Toggle flips the night mode state. Starts or kills hyprsunset accordingly.
func (s *Service) Toggle() {
	newVal := !s.enabled.Load()
	s.enabled.Store(newVal)
	if newVal {
		if err := s.runner.Start("hyprsunset", "-t", s.temp); err != nil {
			log.Printf("nightmode start: %v", err)
		}
	} else {
		if err := s.killer.Kill("hyprsunset"); err != nil {
			log.Printf("nightmode stop: %v", err)
		}
	}
	s.bus.Publish(bus.TopicNightMode, newVal)
}

// Enabled returns the current state.
func (s *Service) Enabled() bool {
	return s.enabled.Load()
}

