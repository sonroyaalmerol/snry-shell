package pomodoro

import (
	"context"
	"sync"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

type phase string

const (
	phaseIdle  phase = "idle"
	phaseWork  phase = "work"
	phaseBreak phase = "break"
)

type Service struct {
	mu       sync.Mutex
	bus      *bus.Bus
	ph       phase
	remaining time.Duration
	running  bool
	sessions int
	cancel   context.CancelFunc
}

const workDuration = 25 * time.Minute
const breakDuration = 5 * time.Minute

func New(b *bus.Bus) *Service {
	return &Service{bus: b, ph: phaseIdle, remaining: workDuration}
}

func (s *Service) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (s *Service) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}
	s.running = true
	if s.ph == phaseIdle {
		s.ph = phaseWork
		s.remaining = workDuration
	}
	childCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.tick(childCtx)
	s.publish()
}

func (s *Service) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.running = false
	s.publish()
}

func (s *Service) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running || s.ph == phaseIdle {
		return
	}
	s.running = true
	childCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.tick(childCtx)
	s.publish()
}

func (s *Service) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.ph = phaseIdle
	s.remaining = workDuration
	s.running = false
	s.publish()
}

func (s *Service) Skip() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.ph == phaseWork {
		s.ph = phaseBreak
		s.remaining = breakDuration
		s.sessions++
	} else {
		s.ph = phaseIdle
		s.remaining = workDuration
	}
	if s.running {
		childCtx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		go s.tick(childCtx)
	} else {
		s.publish()
	}
}

func (s *Service) tick(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			s.remaining -= time.Second
			done := s.remaining <= 0
			if done {
				if s.cancel != nil {
					s.cancel()
					s.cancel = nil
				}
				s.running = false
				if s.ph == phaseWork {
					s.ph = phaseBreak
					s.remaining = breakDuration
					s.sessions++
				} else {
					s.ph = phaseIdle
					s.remaining = workDuration
				}
			}
			s.publish()
			s.mu.Unlock()
		}
	}
}

func (s *Service) publish() {
	s.bus.Publish(bus.TopicPomodoro, state.PomodoroState{
		Phase:             string(s.ph),
		Remaining:         s.remaining,
		Running:           s.running,
		SessionsCompleted: s.sessions,
	})
}
