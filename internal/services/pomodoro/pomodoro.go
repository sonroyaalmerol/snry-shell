package pomodoro

import (
	"context"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
)

// Service provides pomodoro timer state.
type Service struct {
	bus *bus.Bus
}

func New(b *bus.Bus) *Service {
	return &Service{bus: b}
}

func (s *Service) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
