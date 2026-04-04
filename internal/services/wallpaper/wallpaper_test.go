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

const fakeMatugenJSON = `{
	"colors": {
		"dark": {
			"primary": "#BB86FC", "on_primary": "#000000", "primary_container": "#BB86FC", "on_primary_container": "#000000",
			"secondary": "#03DAC6", "on_secondary": "#000000", "secondary_container": "#03DAC6", "on_secondary_container": "#000000",
			"tertiary": "#CF6679", "on_tertiary": "#000000", "tertiary_container": "#CF6679", "on_tertiary_container": "#000000",
			"error": "#CF6679", "on_error": "#000000", "error_container": "#CF6679", "on_error_container": "#000000",
			"surface": "#121212", "surface_dim": "#121212", "surface_bright": "#121212",
			"surface_container": "#1E1E1E", "surface_container_low": "#1E1E1E",
			"surface_container_high": "#2C2C2C", "surface_container_highest": "#2C2C2C",
			"on_surface": "#E6E1E5", "on_surface_variant": "#CAC4D0",
			"background": "#121212", "on_background": "#E6E1E5",
			"outline": "#938F99", "outline_variant": "#49454F"
		}
	}
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
