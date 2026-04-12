package theme

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
)

// createTestPNG writes a small solid-colour PNG to dir and returns its path.
func createTestPNG(t *testing.T, dir string) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			img.SetNRGBA(x, y, color.NRGBA{R: 100, G: 150, B: 200, A: 255})
		}
	}
	path := filepath.Join(dir, "test_wallpaper.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test PNG: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode test PNG: %v", err)
	}
	return path
}

// TestSetWallpaperPersistsSourceToSettings verifies that SetWallpaper saves
// the original source path to settings so it survives a restart.
func TestSetWallpaperPersistsSourceToSettings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	srcPath := createTestPNG(t, dir)

	b := bus.New()
	m := NewMonitor(b)

	if err := m.SetWallpaper(srcPath); err != nil {
		t.Fatalf("SetWallpaper: %v", err)
	}

	cfg, err := settings.Load()
	if err != nil {
		t.Fatalf("Load settings: %v", err)
	}
	if cfg.WallpaperSource != srcPath {
		t.Errorf("WallpaperSource = %q, want %q", cfg.WallpaperSource, srcPath)
	}
}

// TestSetWallpaperPersistsProcessedPath verifies that the processed wallpaper
// path is stored in the key-value store (GetLastWallpaper) after SetWallpaper.
func TestSetWallpaperPersistsProcessedPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	srcPath := createTestPNG(t, dir)

	b := bus.New()
	m := NewMonitor(b)

	if err := m.SetWallpaper(srcPath); err != nil {
		t.Fatalf("SetWallpaper: %v", err)
	}

	last := GetLastWallpaper()
	if last == "" {
		t.Fatal("GetLastWallpaper returned empty after SetWallpaper")
	}
	if _, err := os.Stat(last); err != nil {
		t.Errorf("processed wallpaper file missing at %q: %v", last, err)
	}
}

// TestSetWallpaperPublishesThemeChanged verifies that SetWallpaper publishes
// TopicThemeChanged with the processed (not raw source) path.
func TestSetWallpaperPublishesThemeChanged(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	srcPath := createTestPNG(t, dir)

	b := bus.New()
	var mu sync.Mutex
	var published string

	b.Subscribe(bus.TopicThemeChanged, func(e bus.Event) {
		if p, ok := e.Data.(string); ok {
			mu.Lock()
			published = p
			mu.Unlock()
		}
	})

	m := NewMonitor(b)
	if err := m.SetWallpaper(srcPath); err != nil {
		t.Fatalf("SetWallpaper: %v", err)
	}

	mu.Lock()
	got := published
	mu.Unlock()

	if got == "" {
		t.Fatal("TopicThemeChanged not published after SetWallpaper")
	}
	// The published path must be the processed file, not the raw source.
	if got == srcPath {
		t.Errorf("TopicThemeChanged published raw source path; expected the processed PNG path")
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("published path %q does not exist: %v", got, err)
	}
}

// TestRunRestoresWallpaperOnStartup is the core regression test for the
// persistence bug: after a wallpaper is selected and the shell restarts,
// Run() must re-process and publish TopicThemeChanged so surfaces update.
func TestRunRestoresWallpaperOnStartup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	srcPath := createTestPNG(t, dir)

	// ── Session 1: user selects a wallpaper ───────────────────────────────────
	b1 := bus.New()
	m1 := NewMonitor(b1)
	if err := m1.SetWallpaper(srcPath); err != nil {
		t.Fatalf("SetWallpaper (session 1): %v", err)
	}

	// Confirm it was persisted.
	cfg, _ := settings.Load()
	if cfg.WallpaperSource == "" {
		t.Fatal("WallpaperSource not saved after SetWallpaper in session 1")
	}

	// ── Session 2: shell restarts, new monitor created ────────────────────────
	b2 := bus.New()
	var mu sync.Mutex
	var restoredPath string

	b2.Subscribe(bus.TopicThemeChanged, func(e bus.Event) {
		if p, ok := e.Data.(string); ok {
			mu.Lock()
			restoredPath = p
			mu.Unlock()
		}
	})

	m2 := NewMonitor(b2)
	ctx := t.Context()

	go m2.Run(ctx)

	// Give Run() time to load settings and reprocess.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		p := restoredPath
		mu.Unlock()
		if p != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	got := restoredPath
	mu.Unlock()

	if got == "" {
		t.Fatal("TopicThemeChanged not published on startup — wallpaper will appear black after restart")
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("restored wallpaper path %q does not exist: %v", got, err)
	}
}

// TestRunNoWallpaperSourceIsNoop verifies that Run() does nothing when no
// wallpaper has ever been configured (avoids spurious errors on fresh installs).
func TestRunNoWallpaperSourceIsNoop(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	b := bus.New()
	var mu sync.Mutex
	var changed bool

	b.Subscribe(bus.TopicThemeChanged, func(e bus.Event) {
		mu.Lock()
		changed = true
		mu.Unlock()
	})

	m := NewMonitor(b)
	ctx, cancel := context.WithCancel(context.Background())
	go m.Run(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()

	mu.Lock()
	got := changed
	mu.Unlock()

	if got {
		t.Error("TopicThemeChanged published with no wallpaper configured — should be a no-op")
	}
}
