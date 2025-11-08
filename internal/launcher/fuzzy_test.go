package launcher_test

import (
	"testing"

	"github.com/sonroyaalmerol/snry-shell/internal/launcher"
)

func TestFuzzyScore(t *testing.T) {
	tests := []struct {
		query  string
		target string
		want   int
	}{
		{"fire", "Firefox", 3},    // prefix match (case-insensitive)
		{"fox", "Firefox", 2},     // substring match
		{"frfx", "Firefox", 1},    // subsequence match
		{"xyz", "Firefox", 0},     // no match
		{"", "anything", 1},       // empty query always matches
		{"FIRE", "firefox", 3},    // prefix match case-insensitive
		{"firefox", "Firefox", 3}, // exact (prefix) match
	}
	for _, tt := range tests {
		got := launcher.FuzzyScore(tt.query, tt.target)
		if got != tt.want {
			t.Errorf("FuzzyScore(%q, %q) = %d, want %d", tt.query, tt.target, got, tt.want)
		}
	}
}

func TestFilter(t *testing.T) {
	apps := []launcher.App{
		{Name: "Firefox", Comment: "Web Browser"},
		{Name: "Files", Comment: "File Manager"},
		{Name: "Terminal", Comment: "Terminal emulator"},
		{Name: "foot", Comment: "A fast Wayland terminal"},
	}

	t.Run("prefix match ranks first", func(t *testing.T) {
		results := launcher.Filter(apps, "fi")
		if len(results) == 0 {
			t.Fatal("expected results")
		}
		// Firefox and Files both start with "fi", Terminal/foot don't.
		for _, r := range results {
			if r.Name == "Terminal" {
				t.Fatal("Terminal should not match 'fi'")
			}
		}
	})

	t.Run("empty query returns all", func(t *testing.T) {
		results := launcher.Filter(apps, "")
		if len(results) != len(apps) {
			t.Fatalf("expected %d results, got %d", len(apps), len(results))
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		results := launcher.Filter(apps, "zzzqqq")
		if len(results) != 0 {
			t.Fatalf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("sorted by score descending", func(t *testing.T) {
		results := launcher.Filter(apps, "fire")
		if len(results) == 0 {
			t.Fatal("expected results")
		}
		if results[0].Name != "Firefox" {
			t.Fatalf("expected Firefox first, got %q", results[0].Name)
		}
	})
}
