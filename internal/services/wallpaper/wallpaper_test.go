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

func (f *fakeMatugen) Run(path string) error {
	f.called = append(f.called, path)
	return f.err
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
