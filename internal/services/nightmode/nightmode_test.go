package nightmode_test

import (
	"testing"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/nightmode"
)

type fakeRunner struct {
	started [][]string
}

func (f *fakeRunner) Start(args ...string) error {
	f.started = append(f.started, args)
	return nil
}

type fakeKiller struct {
	killed []string
}

func (f *fakeKiller) Kill(name string) error {
	f.killed = append(f.killed, name)
	return nil
}

func TestNightModeToggle(t *testing.T) {
	b := bus.New()
	gotCh := make(chan bool, 2)
	b.Subscribe(bus.TopicNightMode, func(e bus.Event) {
		gotCh <- e.Data.(bool)
	})

	runner := &fakeRunner{}
	killer := &fakeKiller{}
	svc := nightmode.New(runner, killer, b)

	// First toggle: enable.
	svc.Toggle()
	if !svc.Enabled() {
		t.Fatal("expected enabled after first toggle")
	}
	if len(runner.started) != 1 {
		t.Fatalf("expected 1 start call, got %d", len(runner.started))
	}
	if runner.started[0][0] != "hyprsunset" {
		t.Fatalf("expected hyprsunset, got %v", runner.started[0])
	}
	select {
	case v := <-gotCh:
		if !v {
			t.Fatal("expected true event")
		}
	default:
		t.Fatal("expected nightmode event")
	}

	// Second toggle: disable.
	svc.Toggle()
	if svc.Enabled() {
		t.Fatal("expected disabled after second toggle")
	}
	if len(killer.killed) != 1 || killer.killed[0] != "hyprsunset" {
		t.Fatalf("expected hyprsunset kill, got %v", killer.killed)
	}
	select {
	case v := <-gotCh:
		if v {
			t.Fatal("expected false event")
		}
	default:
		t.Fatal("expected nightmode event")
	}
}

func TestNightModeDefaultOff(t *testing.T) {
	b := bus.New()
	svc := nightmode.New(&fakeRunner{}, &fakeKiller{}, b)
	if svc.Enabled() {
		t.Fatal("expected off by default")
	}
}
