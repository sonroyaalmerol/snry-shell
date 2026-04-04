package controlsocket

import (
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
)

func tmpSocketPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "test.sock")
}

func TestStartAndCleanup(t *testing.T) {
	t.Parallel()
	path := tmpSocketPath(t)

	b := bus.New()
	ln, err := StartAt(b, path)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("socket file not created: %v", err)
	}

	ln.Close()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("socket file not removed after Close")
	}
}

func TestCommandDispatch(t *testing.T) {
	t.Parallel()
	path := tmpSocketPath(t)

	b := bus.New()
	var mu sync.Mutex
	var received string

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if cmd, ok := e.Data.(string); ok {
			mu.Lock()
			received = cmd
			mu.Unlock()
		}
	})

	ln, err := StartAt(b, path)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer ln.Close()

	time.Sleep(10 * time.Millisecond)

	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	_, err = conn.Write([]byte("toggle-overview"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	conn.Close()

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	got := received
	mu.Unlock()
	if got != "toggle-overview" {
		t.Errorf("expected toggle-overview, got %q", got)
	}
}

func TestCommandWithWhitespace(t *testing.T) {
	t.Parallel()
	path := tmpSocketPath(t)

	b := bus.New()
	var mu sync.Mutex
	var received string

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if cmd, ok := e.Data.(string); ok {
			mu.Lock()
			received = cmd
			mu.Unlock()
		}
	})

	ln, err := StartAt(b, path)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer ln.Close()

	time.Sleep(10 * time.Millisecond)

	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	conn.Write([]byte("  toggle-sidebar  \n"))
	conn.Close()

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	got := received
	mu.Unlock()
	if got != "toggle-sidebar" {
		t.Errorf("expected toggle-sidebar (trimmed), got %q", got)
	}
}

func TestEmptyCommand(t *testing.T) {
	t.Parallel()
	path := tmpSocketPath(t)

	b := bus.New()
	var mu sync.Mutex
	var count int

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	ln, err := StartAt(b, path)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer ln.Close()

	time.Sleep(10 * time.Millisecond)

	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	conn.Write([]byte("\n"))
	conn.Close()

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	got := count
	mu.Unlock()
	if got != 0 {
		t.Errorf("expected no dispatch for empty command, got %d", got)
	}
}

func TestMultipleCommands(t *testing.T) {
	t.Parallel()
	path := tmpSocketPath(t)

	b := bus.New()
	var mu sync.Mutex
	var count int

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	ln, err := StartAt(b, path)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer ln.Close()

	time.Sleep(10 * time.Millisecond)

	// Send one command per connection (handler reads once per conn).
	for _, cmd := range []string{"toggle-overview", "toggle-sidebar"} {
		conn, err := net.Dial("unix", path)
		if err != nil {
			t.Fatalf("dial failed: %v", err)
		}
		conn.Write([]byte(cmd + "\n"))
		conn.Close()
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	got := count
	mu.Unlock()
	if got != 2 {
		t.Errorf("expected 2 dispatches, got %d", got)
	}
}

func TestStartFailsOnBadPath(t *testing.T) {
	t.Parallel()
	// Path in a non-existent directory should fail.
	b := bus.New()
	_, err := StartAt(b, "/nonexistent/directory/snry.sock")
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestStartUsesDefaultPath(t *testing.T) {
	t.Parallel()
	b := bus.New()
	ln, err := Start(b)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		ln.Close()
		os.Remove(DefaultPath)
	}()

	if _, err := os.Stat(DefaultPath); err != nil {
		t.Fatalf("default socket file not created: %v", err)
	}
}
