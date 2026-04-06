package settings_test

import (
	"testing"

	"github.com/sonroyaalmerol/snry-shell/internal/settings"
)

func TestDefaultConfig(t *testing.T) {
	cfg := settings.DefaultConfig()
	if cfg.FontScale != 1.0 {
		t.Fatalf("expected font scale 1.0, got %f", cfg.FontScale)
	}
	if cfg.BarPosition != "top" {
		t.Fatalf("expected bar position 'top', got %q", cfg.BarPosition)
	}
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
	original.FontScale = 1.2
	original.BarPosition = "bottom"

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
	if loaded.FontScale != 1.2 {
		t.Fatalf("FontScale not persisted: got %f", loaded.FontScale)
	}
	if loaded.BarPosition != "bottom" {
		t.Fatalf("BarPosition not persisted: got %q", loaded.BarPosition)
	}
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg, err := settings.Load()
	if err != nil {
		t.Fatalf("expected no error for empty store, got: %v", err)
	}
	if cfg.FontScale != 1.0 {
		t.Fatalf("expected default FontScale 1.0, got %f", cfg.FontScale)
	}
	if cfg.BarPosition != "top" {
		t.Fatalf("expected default BarPosition 'top', got %q", cfg.BarPosition)
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
	if loaded.FontScale != 1.0 {
		t.Fatalf("FontScale should be default, got %f", loaded.FontScale)
	}
}
