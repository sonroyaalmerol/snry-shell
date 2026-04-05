package hyprland_test

import (
	"context"
	"strings"
	"testing"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

func runService(t *testing.T, input string, b *bus.Bus) {
	t.Helper()
	r := strings.NewReader(input)
	svc := hyprland.New(hyprland.NewSocketReader(r), b)
	ctx := context.Background()
	if err := svc.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestActiveWindowEvent(t *testing.T) {
	b := bus.New()
	var got state.ActiveWindow
	b.Subscribe(bus.TopicActiveWindow, func(e bus.Event) {
		got = e.Data.(state.ActiveWindow)
	})

	runService(t, "activewindow>>firefox,Mozilla Firefox\n", b)

	if got.Class != "firefox" {
		t.Fatalf("expected class 'firefox', got %q", got.Class)
	}
	if got.Title != "Mozilla Firefox" {
		t.Fatalf("expected title 'Mozilla Firefox', got %q", got.Title)
	}
}

func TestWorkspaceEvent(t *testing.T) {
	b := bus.New()
	var got state.Workspace
	b.Subscribe(bus.TopicWorkspaces, func(e bus.Event) {
		got = e.Data.(state.Workspace)
	})

	runService(t, "workspacev2>>3,code\n", b)

	if got.ID != 3 {
		t.Fatalf("expected id 3, got %d", got.ID)
	}
	if got.Name != "code" {
		t.Fatalf("expected name 'code', got %q", got.Name)
	}
	if !got.Active {
		t.Fatal("expected Active=true")
	}
}

func TestMultipleEvents(t *testing.T) {
	b := bus.New()
	var windows []state.ActiveWindow
	b.Subscribe(bus.TopicActiveWindow, func(e bus.Event) {
		windows = append(windows, e.Data.(state.ActiveWindow))
	})

	input := "activewindow>>foot,Terminal\nactivewindow>>code,editor\n"
	runService(t, input, b)

	if len(windows) != 2 {
		t.Fatalf("expected 2 events, got %d", len(windows))
	}
	if windows[0].Class != "foot" || windows[1].Class != "code" {
		t.Fatalf("unexpected windows: %+v", windows)
	}
}

func TestUnknownEventIgnored(t *testing.T) {
	b := bus.New()
	called := false
	b.Subscribe(bus.TopicActiveWindow, func(e bus.Event) { called = true })
	b.Subscribe(bus.TopicWorkspaces, func(e bus.Event) { called = true })

	runService(t, "monitor>>DP-1\nchangefloating>>0x12345\n", b)

	if called {
		t.Fatal("unknown events should not trigger subscribed topics")
	}
}

func TestFullscreenEvent(t *testing.T) {
	b := bus.New()
	var got bool
	b.Subscribe(bus.TopicFullscreen, func(e bus.Event) {
		got = e.Data.(bool)
	})

	runService(t, "fullscreen>>1\n", b)
	if !got {
		t.Fatal("expected fullscreen=true")
	}

	runService(t, "fullscreen>>0\n", b)
	if got {
		t.Fatal("expected fullscreen=false")
	}
}
