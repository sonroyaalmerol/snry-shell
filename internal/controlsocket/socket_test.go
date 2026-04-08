package controlsocket

import (
	"net"
	"os"
	"path/filepath"
	"strings"
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

// TestSetWallpaperRoutesToSystemControls verifies the fix for the wallpaper
// persistence bug: set-wallpaper: commands must reach TopicSystemControls so
// that themeMonitor.SetWallpaper() is invoked and the source path is saved.
func TestSetWallpaperRoutesToSystemControls(t *testing.T) {
	t.Parallel()
	path := tmpSocketPath(t)

	b := bus.New()
	var mu sync.Mutex
	var gotCmd string

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if cmd, ok := e.Data.(string); ok {
			mu.Lock()
			gotCmd = cmd
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
	conn.Write([]byte("set-wallpaper:/home/user/Pictures/bg.png"))
	conn.Close()

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	got := gotCmd
	mu.Unlock()

	if got != "set-wallpaper:/home/user/Pictures/bg.png" {
		t.Errorf("TopicSystemControls got %q, want set-wallpaper:...", got)
	}
}

// TestSetWallpaperNotPublishedToThemeChanged verifies that set-wallpaper: does
// NOT bypass themeMonitor by publishing a raw path directly to TopicThemeChanged.
func TestSetWallpaperNotPublishedToThemeChanged(t *testing.T) {
	t.Parallel()
	path := tmpSocketPath(t)

	b := bus.New()
	var mu sync.Mutex
	var themeChanged bool

	b.Subscribe(bus.TopicThemeChanged, func(e bus.Event) {
		mu.Lock()
		themeChanged = true
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
	conn.Write([]byte("set-wallpaper:/home/user/Pictures/bg.png"))
	conn.Close()

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	got := themeChanged
	mu.Unlock()

	if got {
		t.Error("set-wallpaper: must not publish directly to TopicThemeChanged (bypasses themeMonitor.SetWallpaper)")
	}
}

// TestLongPathNotTruncated verifies that file paths longer than the old 256-byte
// buffer are delivered intact to TopicSystemControls.
func TestLongPathNotTruncated(t *testing.T) {
	t.Parallel()
	sockPath := tmpSocketPath(t)

	// Build a wallpaper command with a path longer than the old 256-byte buffer.
	longFilePath := "/home/user/Pictures/" + strings.Repeat("a", 280)
	cmd := "set-wallpaper:" + longFilePath

	b := bus.New()
	var mu sync.Mutex
	var gotCmd string

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if c, ok := e.Data.(string); ok {
			mu.Lock()
			gotCmd = c
			mu.Unlock()
		}
	})

	ln, err := StartAt(b, sockPath)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer ln.Close()

	time.Sleep(10 * time.Millisecond)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	conn.Write([]byte(cmd))
	conn.Close()

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	got := gotCmd
	mu.Unlock()

	if got != cmd {
		t.Errorf("long path truncated or lost:\n got  %q (len %d)\n want %q (len %d)", got, len(got), cmd, len(cmd))
	}
}
