package wallpaper_test

import (
	"context"
	"testing"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/wallpaper"
)

type fakeWatcher struct {
	paths []string
}

func (f *fakeWatcher) Watch(ctx context.Context) (<-chan string, error) {
	ch := make(chan string, len(f.paths))
	for _, p := range f.paths {
		ch <- p
	}
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

type fakeMatugen struct {
	called []string
	err    error
}

// fakeMatugenJSON matches matugen v4 output format.
const fakeMatugenJSON = `{
	"colors": {
		"primary": {"dark": {"color": "#BB86FC"}, "default": {"color": "#BB86FC"}, "light": {"color": "#6750A4"}},
		"on_primary": {"dark": {"color": "#000000"}, "default": {"color": "#000000"}, "light": {"color": "#FFFFFF"}},
		"primary_container": {"dark": {"color": "#BB86FC"}, "default": {"color": "#BB86FC"}, "light": {"color": "#EADDFF"}},
		"on_primary_container": {"dark": {"color": "#000000"}, "default": {"color": "#000000"}, "light": {"color": "#21005D"}},
		"secondary": {"dark": {"color": "#03DAC6"}, "default": {"color": "#03DAC6"}, "light": {"color": "#625B71"}},
		"on_secondary": {"dark": {"color": "#000000"}, "default": {"color": "#000000"}, "light": {"color": "#FFFFFF"}},
		"secondary_container": {"dark": {"color": "#03DAC6"}, "default": {"color": "#03DAC6"}, "light": {"color": "#E8DEF8"}},
		"on_secondary_container": {"dark": {"color": "#000000"}, "default": {"color": "#000000"}, "light": {"color": "#1D192B"}},
		"tertiary": {"dark": {"color": "#CF6679"}, "default": {"color": "#CF6679"}, "light": {"color": "#7D5260"}},
		"on_tertiary": {"dark": {"color": "#000000"}, "default": {"color": "#000000"}, "light": {"color": "#FFFFFF"}},
		"tertiary_container": {"dark": {"color": "#CF6679"}, "default": {"color": "#CF6679"}, "light": {"color": "#FFD8E4"}},
		"on_tertiary_container": {"dark": {"color": "#000000"}, "default": {"color": "#000000"}, "light": {"color": "#31111D"}},
		"error": {"dark": {"color": "#CF6679"}, "default": {"color": "#CF6679"}, "light": {"color": "#B3261E"}},
		"on_error": {"dark": {"color": "#000000"}, "default": {"color": "#000000"}, "light": {"color": "#FFFFFF"}},
		"error_container": {"dark": {"color": "#CF6679"}, "default": {"color": "#CF6679"}, "light": {"color": "#F9DEDC"}},
		"on_error_container": {"dark": {"color": "#000000"}, "default": {"color": "#000000"}, "light": {"color": "#410E0B"}},
		"surface": {"dark": {"color": "#121212"}, "default": {"color": "#121212"}, "light": {"color": "#FFFBFE"}},
		"surface_dim": {"dark": {"color": "#121212"}, "default": {"color": "#121212"}, "light": {"color": "#DED8E1"}},
		"surface_bright": {"dark": {"color": "#121212"}, "default": {"color": "#121212"}, "light": {"color": "#FFFBFE"}},
		"surface_container": {"dark": {"color": "#1E1E1E"}, "default": {"color": "#1E1E1E"}, "light": {"color": "#F3EDF7"}},
		"surface_container_low": {"dark": {"color": "#1E1E1E"}, "default": {"color": "#1E1E1E"}, "light": {"color": "#F7F2FA"}},
		"surface_container_high": {"dark": {"color": "#2C2C2C"}, "default": {"color": "#2C2C2C"}, "light": {"color": "#ECE6F0"}},
		"surface_container_highest": {"dark": {"color": "#2C2C2C"}, "default": {"color": "#2C2C2C"}, "light": {"color": "#E6E0E9"}},
		"on_surface": {"dark": {"color": "#E6E1E5"}, "default": {"color": "#E6E1E5"}, "light": {"color": "#1C1B1F"}},
		"on_surface_variant": {"dark": {"color": "#CAC4D0"}, "default": {"color": "#CAC4D0"}, "light": {"color": "#49454F"}},
		"background": {"dark": {"color": "#121212"}, "default": {"color": "#121212"}, "light": {"color": "#FFFBFE"}},
		"on_background": {"dark": {"color": "#E6E1E5"}, "default": {"color": "#E6E1E5"}, "light": {"color": "#1C1B1F"}},
		"outline": {"dark": {"color": "#938F99"}, "default": {"color": "#938F99"}, "light": {"color": "#79747E"}},
		"outline_variant": {"dark": {"color": "#49454F"}, "default": {"color": "#49454F"}, "light": {"color": "#CAC4D0"}}
	},
	"mode": "dark",
	"is_dark_mode": true
}`

func (f *fakeMatugen) Run(path string) ([]byte, error) {
	f.called = append(f.called, path)
	if f.err != nil {
		return nil, f.err
	}
	return []byte(fakeMatugenJSON), nil
}

func TestServiceTriggersMatugenOnChange(t *testing.T) {
	b := bus.New()
	themeCh := make(chan struct{}, 4)
	b.Subscribe(bus.TopicTheme, func(_ bus.Event) { themeCh <- struct{}{} })

	watcher := &fakeWatcher{paths: []string{"/wallpapers/forest.jpg", "/wallpapers/city.jpg"}}
	mg := &fakeMatugen{}
	svc := wallpaper.NewWithDeps(watcher, mg, b)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	svc.Run(ctx) //nolint:errcheck

	if len(mg.called) != 2 {
		t.Fatalf("expected matugen called 2 times, got %d", len(mg.called))
	}
	if mg.called[0] != "/wallpapers/forest.jpg" {
		t.Fatalf("unexpected path: %q", mg.called[0])
	}
	if len(themeCh) != 2 {
		t.Fatalf("expected 2 theme events, got %d", len(themeCh))
	}
}

func TestServiceSkipsOnMatugenError(t *testing.T) {
	b := bus.New()
	themeCh := make(chan struct{}, 4)
	b.Subscribe(bus.TopicTheme, func(_ bus.Event) { themeCh <- struct{}{} })

	watcher := &fakeWatcher{paths: []string{"/wallpapers/broken.jpg"}}
	mg := &fakeMatugen{err: context.DeadlineExceeded}
	svc := wallpaper.NewWithDeps(watcher, mg, b)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	svc.Run(ctx) //nolint:errcheck

	if len(themeCh) != 0 {
		t.Fatal("expected no theme events when matugen fails")
	}
}
