package settings_test

import (
	"testing"

	"github.com/sonroyaalmerol/snry-shell/internal/settings"
)

func TestDefaultConfig(t *testing.T) {
	cfg := settings.DefaultConfig()
	if !cfg.DarkMode {
		t.Fatal("expected dark mode enabled by default")
	}
	if cfg.DoNotDisturb {
		t.Fatal("expected do-not-disturb off by default")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	original := settings.DefaultConfig()
	original.DoNotDisturb = true

	if err := settings.Save(original); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := settings.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !loaded.DoNotDisturb {
		t.Fatal("DoNotDisturb not persisted")
	}
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg, err := settings.Load()
	if err != nil {
		t.Fatalf("expected no error for empty store, got: %v", err)
	}
	if !cfg.DarkMode {
		t.Fatalf("expected default DarkMode true, got %v", cfg.DarkMode)
	}
}

func TestWallpaperSourcePersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := settings.DefaultConfig()
	cfg.WallpaperSource = "/home/user/Pictures/wallpaper.jpg"
	if err := settings.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := settings.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.WallpaperSource != cfg.WallpaperSource {
		t.Errorf("WallpaperSource = %q, want %q", loaded.WallpaperSource, cfg.WallpaperSource)
	}
}

// TestWallpaperSourceNotOverwrittenByOtherSave verifies that saving other
// settings after a wallpaper is selected does not clobber WallpaperSource.
// This guards against the bug where the control panel's cfg copy had an empty
// WallpaperSource and a subsequent Save() would overwrite the persisted value.
func TestWallpaperSourceNotOverwrittenByOtherSave(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Simulate themeMonitor.SetWallpaper() persisting the source.
	cfgWithSource := settings.DefaultConfig()
	cfgWithSource.WallpaperSource = "/home/user/Pictures/bg.jpg"
	if err := settings.Save(cfgWithSource); err != nil {
		t.Fatalf("Save (with source): %v", err)
	}

	// Simulate the control panel saving a different field (e.g. blur) while
	// keeping WallpaperSource populated (as the fix in provider_wallpaper.go ensures).
	cfgBlurUpdate, err := settings.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfgBlurUpdate.WallpaperBlur = 10
	if err := settings.Save(cfgBlurUpdate); err != nil {
		t.Fatalf("Save (blur update): %v", err)
	}

	// WallpaperSource must still be intact.
	final, err := settings.Load()
	if err != nil {
		t.Fatalf("Load final: %v", err)
	}
	if final.WallpaperSource != "/home/user/Pictures/bg.jpg" {
		t.Errorf("WallpaperSource lost after blur save: got %q, want %q",
			final.WallpaperSource, "/home/user/Pictures/bg.jpg")
	}
	if final.WallpaperBlur != 10 {
		t.Errorf("WallpaperBlur = %d, want 10", final.WallpaperBlur)
	}
}

func TestIndividualFieldPersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Save only one field by saving a config with defaults except one.
	cfg := settings.DefaultConfig()
	cfg.DoNotDisturb = true
	if err := settings.Save(cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := settings.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.DoNotDisturb {
		t.Fatal("DoNotDisturb should be persisted")
	}
	// Other fields should still match defaults.
	if !loaded.DarkMode {
		t.Fatalf("DarkMode should be default true, got %v", loaded.DarkMode)
	}
}
