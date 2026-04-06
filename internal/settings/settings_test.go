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
