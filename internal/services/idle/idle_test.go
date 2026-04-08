package idle

import (
	"context"
	"testing"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.LockTimeout != 5*time.Minute {
		t.Errorf("expected 5m lock timeout, got %v", cfg.LockTimeout)
	}
	if cfg.IdleDisplayOffTimeout != 2*time.Minute {
		t.Errorf("expected 2m display off timeout, got %v", cfg.IdleDisplayOffTimeout)
	}
	if cfg.SuspendTimeout != 0 {
		t.Errorf("expected 0 suspend timeout, got %v", cfg.SuspendTimeout)
	}
}

func TestUpdateConfig(t *testing.T) {
	b := bus.New()
	svc := New(b, nil, DefaultConfig())

	newCfg := Config{
		LockTimeout:    10 * time.Minute,
		SuspendTimeout: 5 * time.Minute,
	}

	svc.UpdateConfig(newCfg)

	svc.mu.Lock()
	defer svc.mu.Unlock()

	if svc.cfg.LockTimeout != 10*time.Minute {
		t.Errorf("expected 10m lock timeout, got %v", svc.cfg.LockTimeout)
	}
	if svc.cfg.SuspendTimeout != 5*time.Minute {
		t.Errorf("expected 5m suspend timeout, got %v", svc.cfg.SuspendTimeout)
	}
}

func TestDoLock(t *testing.T) {
	b := bus.New()
	svc := New(b, nil, DefaultConfig())

	lockedEvent := false
	b.Subscribe(bus.TopicScreenLock, func(e bus.Event) {
		ls, ok := e.Data.(state.LockScreenState)
		if ok && ls.Locked {
			lockedEvent = true
		}
	})

	svc.doLock()

	if !lockedEvent {
		t.Errorf("expected TopicScreenLock event to be published with Locked=true")
	}
}

func TestScreenLockBusEvent(t *testing.T) {
	b := bus.New()
	svc := New(b, nil, DefaultConfig())

	// Start a dummy run loop just to get the subscriptions
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go svc.Run(ctx)

	// Wait briefly for goroutine to start
	time.Sleep(50 * time.Millisecond)

	// Simulate locking the screen
	b.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: true})

	// Wait briefly for bus event to be processed
	time.Sleep(50 * time.Millisecond)

	svc.mu.Lock()
	if !svc.locked {
		t.Errorf("expected locked to be true after receiving lock event")
	}
	if svc.idleStarted.IsZero() {
		t.Errorf("expected idleStarted to be set after receiving lock event")
	}
	svc.mu.Unlock()

	// Simulate dismissing the lockscreen
	b.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: false})

	// Wait briefly for bus event to be processed
	time.Sleep(50 * time.Millisecond)

	svc.mu.Lock()
	defer svc.mu.Unlock()

	if svc.locked {
		t.Errorf("expected locked to be false after receiving unlock event")
	}
	if !svc.idleStarted.IsZero() {
		t.Errorf("expected idleStarted to be zeroed after receiving unlock event")
	}
}
