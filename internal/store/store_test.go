package store_test

import (
	"testing"

	"github.com/sonroyaalmerol/snry-shell/internal/store"
)

func TestSetAndGet(t *testing.T) {
	store.Reset()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if err := store.Set("greeting", "hello"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	v, ok := store.Lookup[string]("greeting")
	if !ok {
		t.Fatal("expected key to be found")
	}
	if v != "hello" {
		t.Fatalf("got %q, want %q", v, "hello")
	}
}

func TestLookupOr(t *testing.T) {
	store.Reset()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	got := store.LookupOr("missing", 42)
	if got != 42 {
		t.Fatalf("expected default 42, got %d", got)
	}
}

func TestSetBool(t *testing.T) {
	store.Reset()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if err := store.Set("flag", true); err != nil {
		t.Fatalf("Set: %v", err)
	}
	v, ok := store.Lookup[bool]("flag")
	if !ok || !v {
		t.Fatal("expected true")
	}
}

func TestDelete(t *testing.T) {
	store.Reset()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	_ = store.Set("tmp", "value")
	if err := store.Delete("tmp"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, ok := store.Lookup[string]("tmp")
	if ok {
		t.Fatal("expected key to be absent after Delete")
	}
}

func TestDeleteMissing(t *testing.T) {
	store.Reset()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if err := store.Delete("nonexistent"); err != nil {
		t.Fatalf("Delete of missing key should not error: %v", err)
	}
}

func TestKeys(t *testing.T) {
	store.Reset()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	_ = store.Set("b", 2)
	_ = store.Set("a", 1)
	_ = store.Set("c", 3)

	keys := store.Keys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Fatalf("keys not sorted: %v", keys)
	}
}

func TestPersistenceAcrossLoads(t *testing.T) {
	store.Reset()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if err := store.Set("count", 7); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Simulate a new session by reading via Get (reads from disk).
	var v int
	if !store.Get("count", &v) {
		t.Fatal("expected key to persist")
	}
	if v != 7 {
		t.Fatalf("got %d, want 7", v)
	}
}

func TestOverwrite(t *testing.T) {
	store.Reset()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	_ = store.Set("x", "first")
	_ = store.Set("x", "second")

	v := store.LookupOr("x", "")
	if v != "second" {
		t.Fatalf("expected overwritten value %q, got %q", "second", v)
	}
}

func TestSetMany(t *testing.T) {
	store.Reset()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if err := store.SetMany(map[string]any{"p": 1, "q": "two", "r": true}); err != nil {
		t.Fatalf("SetMany: %v", err)
	}

	if store.LookupOr("p", 0) != 1 {
		t.Fatal("p not persisted")
	}
	if store.LookupOr("q", "") != "two" {
		t.Fatal("q not persisted")
	}
	if !store.LookupOr("r", false) {
		t.Fatal("r not persisted")
	}
}

func TestMissingFileReturnsDefault(t *testing.T) {
	store.Reset()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	keys := store.Keys()
	if len(keys) != 0 {
		t.Fatalf("expected empty store, got keys: %v", keys)
	}
}
