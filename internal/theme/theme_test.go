package theme

import (
	"testing"
)

func TestRgbToHsl(t *testing.T) {
	tests := []struct {
		r, g, b uint8
		wantH   float64
		wantS   float64
		wantL   float64
		tol     float64
	}{
		{255, 0, 0, 0, 1.0, 0.5, 0.01},   // Red
		{0, 255, 0, 120, 1.0, 0.5, 0.01}, // Green
		{0, 0, 255, 240, 1.0, 0.5, 0.01}, // Blue
		{255, 255, 255, 0, 0, 1.0, 0.01}, // White
		{0, 0, 0, 0, 0, 0, 0.01},         // Black
		{128, 128, 128, 0, 0, 0.5, 0.01}, // Gray
	}

	for _, tt := range tests {
		// Create color.RGBA from test values
		c := &mockColor{r: uint32(tt.r) * 257, g: uint32(tt.g) * 257, b: uint32(tt.b) * 257}
		h, s, l := rgbToHsl(c)

		if diff := abs(h - tt.wantH); diff > tt.tol && diff < 360-tt.tol {
			t.Errorf("rgbToHsl(%d,%d,%d) h=%v, want %v", tt.r, tt.g, tt.b, h, tt.wantH)
		}
		if abs(s-tt.wantS) > tt.tol {
			t.Errorf("rgbToHsl(%d,%d,%d) s=%v, want %v", tt.r, tt.g, tt.b, s, tt.wantS)
		}
		if abs(l-tt.wantL) > tt.tol {
			t.Errorf("rgbToHsl(%d,%d,%d) l=%v, want %v", tt.r, tt.g, tt.b, l, tt.wantL)
		}
	}
}

func TestHslToHex(t *testing.T) {
	tests := []struct {
		h, s, l float64
		want    string
	}{
		{0, 1.0, 0.5, "#FF0000"},   // Red
		{120, 1.0, 0.5, "#00FF00"}, // Green
		{240, 1.0, 0.5, "#0000FF"}, // Blue
		{0, 0, 1.0, "#FFFFFF"},     // White
		{0, 0, 0, "#000000"},       // Black
	}

	for _, tt := range tests {
		got := hslToHex(tt.h, tt.s, tt.l)
		if got != tt.want {
			t.Errorf("hslToHex(%v,%v,%v) = %s, want %s", tt.h, tt.s, tt.l, got, tt.want)
		}
	}
}

type mockColor struct {
	r, g, b, a uint32
}

func (m *mockColor) RGBA() (r, g, b, a uint32) {
	return m.r, m.g, m.b, 0xFFFF
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestGenerateCSS(t *testing.T) {
	scheme := &ColorScheme{
		Primary:    "#BB86FC",
		OnPrimary:  "#000000",
		Surface:    "#1E1E2E",
		OnSurface:  "#CDD6F4",
		Background: "#11111B",
	}

	css := generateCSS(scheme)

	if css == "" {
		t.Error("generateCSS returned empty string")
	}

	expectedTokens := []string{
		"@define-color col_primary #BB86FC",
		"@define-color col_on_primary #000000",
		"@define-color col_surface #1E1E2E",
	}

	for _, token := range expectedTokens {
		if !contains(css, token) {
			t.Errorf("generateCSS missing expected token: %s", token)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
