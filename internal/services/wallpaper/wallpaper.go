package wallpaper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/theme"
)

// FileWatcher abstracts watching a file for changes.
type FileWatcher interface {
	// Watch returns a channel that receives the new file path each time the
	// wallpaper changes. The channel is closed when ctx is done.
	Watch(ctx context.Context) (<-chan string, error)
}

// MatugenRunner abstracts running matugen for testability.
type MatugenRunner interface {
	Run(wallpaperPath string) ([]byte, error)
}

type realFileWatcher struct {
	statePath string
	interval  time.Duration
}

// NewFileWatcher returns a FileWatcher that polls the swww state file.
func NewFileWatcher() FileWatcher {
	return &realFileWatcher{
		statePath: swwwStatePath(),
		interval:  2 * time.Second,
	}
}

func swwwStatePath() string {
	xdg := os.Getenv("XDG_RUNTIME_DIR")
	if xdg == "" {
		xdg = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return filepath.Join(xdg, "swww", "swww.socket")
}

func (w *realFileWatcher) Watch(ctx context.Context) (<-chan string, error) {
	ch := make(chan string, 1)
	go func() {
		defer close(ch)
		var last string
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				path, err := currentWallpaper()
				if err != nil || path == last {
					continue
				}
				last = path
				ch <- path
			}
		}
	}()
	return ch, nil
}

// currentWallpaper queries swww for the active wallpaper path.
func currentWallpaper() (string, error) {
	out, err := exec.Command("swww", "query").Output()
	if err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		// Format: "monitor: <name>, image: <path>"
		if idx := strings.Index(line, "image: "); idx != -1 {
			return strings.TrimSpace(line[idx+7:]), nil
		}
	}
	return "", io.ErrUnexpectedEOF
}

type execMatugen struct{}

func (e execMatugen) Run(wallpaperPath string) ([]byte, error) {
	return exec.Command("matugen", "image", wallpaperPath, "--json", "hex").Output()
}

// NewMatugenRunner returns a MatugenRunner backed by the real matugen binary.
func NewMatugenRunner() MatugenRunner { return execMatugen{} }

// Service watches for wallpaper changes and triggers matugen + publishes theme events.
type Service struct {
	watcher  FileWatcher
	matugen  MatugenRunner
	bus      *bus.Bus
}

func New(b *bus.Bus) *Service {
	return &Service{
		watcher: NewFileWatcher(),
		matugen: NewMatugenRunner(),
		bus:     b,
	}
}

func NewWithDeps(watcher FileWatcher, matugen MatugenRunner, b *bus.Bus) *Service {
	return &Service{watcher: watcher, matugen: matugen, bus: b}
}

func (s *Service) Run(ctx context.Context) error {
	ch, err := s.watcher.Watch(ctx)
	if err != nil {
		return fmt.Errorf("wallpaper watcher: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case path, ok := <-ch:
			if !ok {
				return nil
			}
			s.handleChange(path)
		}
	}
}

func (s *Service) handleChange(wallpaperPath string) {
	scheme, err := theme.GenerateFromWallpaper(s.matugen, wallpaperPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wallpaper: theme generation: %v\n", err)
		return
	}
	if err := theme.WriteCSS(scheme, themeCachePath()); err != nil {
		fmt.Fprintf(os.Stderr, "wallpaper: write theme: %v\n", err)
		return
	}
	s.bus.Publish(bus.TopicTheme, scheme)
}

func themeCachePath() string {
	home, _ := os.UserCacheDir()
	return filepath.Join(home, "snry-shell", "theme.css")
}
