package theme_test

import (
	"strings"
	"testing"

	"github.com/sonroyaalmerol/snry-shell/internal/state"
	"github.com/sonroyaalmerol/snry-shell/internal/theme"
)

func fullScheme() state.ColorScheme {
	return state.ColorScheme{
		Primary:            "#6750A4",
		OnPrimary:          "#FFFFFF",
		PrimaryContainer:   "#EADDFF",
		OnPrimaryContainer: "#21005D",

		Secondary:            "#625B71",
		OnSecondary:          "#FFFFFF",
		SecondaryContainer:   "#E8DEF8",
		OnSecondaryContainer: "#1D192B",

		Tertiary:            "#7D5260",
		OnTertiary:          "#FFFFFF",
		TertiaryContainer:   "#FFD8E4",
		OnTertiaryContainer: "#31111D",

		Error:            "#B3261E",
		OnError:          "#FFFFFF",
		ErrorContainer:   "#F9DEDC",
		OnErrorContainer: "#410E0B",

		Surface:                 "#1C1B1F",
		SurfaceDim:              "#141218",
		SurfaceBright:           "#3B383E",
		SurfaceContainer:        "#211F26",
		SurfaceContainerLow:     "#1D1B20",
		SurfaceContainerHigh:    "#2B2930",
		SurfaceContainerHighest: "#36343B",
		OnSurface:               "#E6E1E5",
		OnSurfaceVariant:        "#CAC4D0",

		Background:   "#1C1B1F",
		OnBackground: "#E6E1E5",

		Outline:        "#938F99",
		OutlineVariant: "#49454F",

		Subtext: "#CAC4D0",
	}
}

func TestRenderCSS(t *testing.T) {
	scheme := fullScheme()
	css := theme.RenderCSS(scheme)

	checks := []string{
		"@define-color col_primary #6750A4",
		"@define-color col_on_primary #FFFFFF",
		"@define-color col_primary_container #EADDFF",
		"@define-color col_on_primary_container #21005D",
		"@define-color col_secondary #625B71",
		"@define-color col_secondary_container #E8DEF8",
		"@define-color col_error #B3261E",
		"@define-color col_error_container #F9DEDC",
		"@define-color col_surface #1C1B1F",
		"@define-color col_surface_dim #141218",
		"@define-color col_surface_container #211F26",
		"@define-color col_surface_container_high #2B2930",
		"@define-color col_surface_container_highest #36343B",
		"@define-color col_on_surface #E6E1E5",
		"@define-color col_on_surface_variant #CAC4D0",
		"@define-color col_background #1C1B1F",
		"@define-color col_outline #938F99",
		"@define-color col_outline_variant #49454F",
		"@define-color col_subtext #CAC4D0",
	}
	for _, want := range checks {
		if !strings.Contains(css, want) {
			t.Errorf("missing %q in CSS output", want)
		}
	}
}

type fakeMatugen struct {
	response []byte
	err      error
}

func (f *fakeMatugen) Run(_ string) ([]byte, error) {
	return f.response, f.err
}

func TestGenerateFromWallpaper(t *testing.T) {
	raw := `{
		"colors": {
			"dark": {
				"primary": "#aabbcc",
				"on_primary": "#ffffff",
				"primary_container": "#ddeeff",
				"on_primary_container": "#001122",
				"secondary": "#bbccdd",
				"on_secondary": "#ffffff",
				"secondary_container": "#ccdde e",
				"on_secondary_container": "#112233",
				"tertiary": "#ccddee",
				"on_tertiary": "#ffffff",
				"tertiary_container": "#ddeeff",
				"on_tertiary_container": "#223344",
				"error": "#ff0000",
				"on_error": "#ffffff",
				"error_container": "#ffcccc",
				"on_error_container": "#330000",
				"surface": "#111111",
				"surface_dim": "#0a0a0a",
				"surface_bright": "#333333",
				"surface_container": "#1a1a1a",
				"surface_container_low": "#151515",
				"surface_container_high": "#202020",
				"surface_container_highest": "#2a2a2a",
				"on_surface": "#eeeeee",
				"on_surface_variant": "#cccccc",
				"background": "#000000",
				"on_background": "#ffffff",
				"outline": "#888888",
				"outline_variant": "#444444"
			},
			"light": {}
		}
	}`
	runner := &fakeMatugen{response: []byte(raw)}
	scheme, err := theme.GenerateFromWallpaper(runner, "/fake/wall.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if scheme.Primary != "#aabbcc" {
		t.Fatalf("unexpected primary: %q", scheme.Primary)
	}
	if scheme.Error != "#ff0000" {
		t.Fatalf("unexpected error: %q", scheme.Error)
	}
	if scheme.SurfaceContainerHighest != "#2a2a2a" {
		t.Fatalf("unexpected surface_container_highest: %q", scheme.SurfaceContainerHighest)
	}
	if scheme.OnSurfaceVariant != "#cccccc" {
		t.Fatalf("unexpected on_surface_variant: %q", scheme.OnSurfaceVariant)
	}
	// Subtext is alias of on_surface_variant
	if scheme.Subtext != scheme.OnSurfaceVariant {
		t.Fatalf("subtext should equal on_surface_variant")
	}
}

func TestGenerateFromWallpaperInvalidJSON(t *testing.T) {
	runner := &fakeMatugen{response: []byte(`not json`)}
	_, err := theme.GenerateFromWallpaper(runner, "/fake/wall.jpg")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGenerateFromWallpaperRunnerError(t *testing.T) {
	runner := &fakeMatugen{err: &runnerError{"matugen not found"}}
	_, err := theme.GenerateFromWallpaper(runner, "/fake/wall.jpg")
	if err == nil {
		t.Fatal("expected error when runner fails")
	}
}

type runnerError struct{ msg string }

func (e *runnerError) Error() string { return e.msg }
