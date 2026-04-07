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
	if cfg.DisplayOffTimeout != 30*time.Second {
		t.Errorf("expected 30s display off timeout, got %v", cfg.DisplayOffTimeout)
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

	svc.mu.Lock()
	if !svc.locked {
		t.Errorf("expected locked to be true")
	}
	if svc.idleStarted.IsZero() {
		t.Errorf("expected idleStarted to be set")
	}
	svc.mu.Unlock()

	if !lockedEvent {
		t.Errorf("expected TopicScreenLock event to be published with Locked=true")
	}

	// Calling doLock again shouldn't change idleStarted
	svc.mu.Lock()
	firstTime := svc.idleStarted
	svc.mu.Unlock()

	svc.doLock()

	svc.mu.Lock()
	if svc.idleStarted != firstTime {
		t.Errorf("expected idleStarted to remain unchanged on subsequent doLock calls")
	}
	svc.mu.Unlock()
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

	// Set locked state
	svc.mu.Lock()
	svc.locked = true
	svc.idleStarted = time.Now()
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

func TestMin(t *testing.T) {
	if min(1, 2) != 1 {
		t.Errorf("expected min(1, 2) to be 1")
	}
	if min(5, 3) != 3 {
		t.Errorf("expected min(5, 3) to be 3")
	}
	if min(4, 4) != 4 {
		t.Errorf("expected min(4, 4) to be 4")
	}
}
