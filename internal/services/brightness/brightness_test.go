package brightness_test

import (
	"testing"

	"github.com/sonroyaalmerol/snry-shell/internal/services/brightness"
)

func TestParseBrightnessctl(t *testing.T) {
	tests := []struct {
		current string
		max     string
		wantCur int
		wantMax int
	}{
		{"100\n", "255\n", 100, 255},
		{"0\n", "255\n", 0, 255},
		{"255\n", "255\n", 255, 255},
		{"120", "255", 120, 255},
	}
	for _, tt := range tests {
		got, err := brightness.ParseBrightnessctl(tt.current, tt.max)
		if err != nil {
			t.Fatalf("current=%q max=%q: unexpected error: %v", tt.current, tt.max, err)
		}
		if got.Current != tt.wantCur {
			t.Fatalf("current=%q: expected %d, got %d", tt.current, tt.wantCur, got.Current)
		}
		if got.Max != tt.wantMax {
			t.Fatalf("max=%q: expected %d, got %d", tt.max, tt.wantMax, got.Max)
		}
	}
}

func TestParseBrightnessctlInvalid(t *testing.T) {
	_, err := brightness.ParseBrightnessctl("not a number", "255")
	if err == nil {
		t.Fatal("expected error for invalid current")
	}
	_, err = brightness.ParseBrightnessctl("100", "not a number")
	if err == nil {
		t.Fatal("expected error for invalid max")
	}
}
