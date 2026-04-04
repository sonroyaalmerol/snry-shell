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
			"primary": {"dark": {"color": "#aabbcc"}, "default": {"color": "#aabbcc"}, "light": {"color": "#6750A4"}},
			"on_primary": {"dark": {"color": "#ffffff"}, "default": {"color": "#ffffff"}, "light": {"color": "#FFFFFF"}},
			"primary_container": {"dark": {"color": "#ddeeff"}, "default": {"color": "#ddeeff"}, "light": {"color": "#EADDFF"}},
			"on_primary_container": {"dark": {"color": "#001122"}, "default": {"color": "#001122"}, "light": {"color": "#21005D"}},
			"secondary": {"dark": {"color": "#bbccdd"}, "default": {"color": "#bbccdd"}, "light": {"color": "#625B71"}},
			"on_secondary": {"dark": {"color": "#ffffff"}, "default": {"color": "#ffffff"}, "light": {"color": "#FFFFFF"}},
			"secondary_container": {"dark": {"color": "#ccddee"}, "default": {"color": "#ccddee"}, "light": {"color": "#E8DEF8"}},
			"on_secondary_container": {"dark": {"color": "#112233"}, "default": {"color": "#112233"}, "light": {"color": "#1D192B"}},
			"tertiary": {"dark": {"color": "#ccddee"}, "default": {"color": "#ccddee"}, "light": {"color": "#7D5260"}},
			"on_tertiary": {"dark": {"color": "#ffffff"}, "default": {"color": "#ffffff"}, "light": {"color": "#FFFFFF"}},
			"tertiary_container": {"dark": {"color": "#ddeeff"}, "default": {"color": "#ddeeff"}, "light": {"color": "#FFD8E4"}},
			"on_tertiary_container": {"dark": {"color": "#223344"}, "default": {"color": "#223344"}, "light": {"color": "#31111D"}},
			"error": {"dark": {"color": "#ff0000"}, "default": {"color": "#ff0000"}, "light": {"color": "#B3261E"}},
			"on_error": {"dark": {"color": "#ffffff"}, "default": {"color": "#ffffff"}, "light": {"color": "#FFFFFF"}},
			"error_container": {"dark": {"color": "#ffcccc"}, "default": {"color": "#ffcccc"}, "light": {"color": "#F9DEDC"}},
			"on_error_container": {"dark": {"color": "#330000"}, "default": {"color": "#330000"}, "light": {"color": "#410E0B"}},
			"surface": {"dark": {"color": "#111111"}, "default": {"color": "#111111"}, "light": {"color": "#FFFBFE"}},
			"surface_dim": {"dark": {"color": "#0a0a0a"}, "default": {"color": "#0a0a0a"}, "light": {"color": "#DED8E1"}},
			"surface_bright": {"dark": {"color": "#333333"}, "default": {"color": "#333333"}, "light": {"color": "#FFFBFE"}},
			"surface_container": {"dark": {"color": "#1a1a1a"}, "default": {"color": "#1a1a1a"}, "light": {"color": "#F3EDF7"}},
			"surface_container_low": {"dark": {"color": "#151515"}, "default": {"color": "#151515"}, "light": {"color": "#F7F2FA"}},
			"surface_container_high": {"dark": {"color": "#202020"}, "default": {"color": "#202020"}, "light": {"color": "#ECE6F0"}},
			"surface_container_highest": {"dark": {"color": "#2a2a2a"}, "default": {"color": "#2a2a2a"}, "light": {"color": "#E6E0E9"}},
			"on_surface": {"dark": {"color": "#eeeeee"}, "default": {"color": "#eeeeee"}, "light": {"color": "#1C1B1F"}},
			"on_surface_variant": {"dark": {"color": "#cccccc"}, "default": {"color": "#cccccc"}, "light": {"color": "#49454F"}},
			"background": {"dark": {"color": "#000000"}, "default": {"color": "#000000"}, "light": {"color": "#FFFBFE"}},
			"on_background": {"dark": {"color": "#ffffff"}, "default": {"color": "#ffffff"}, "light": {"color": "#1C1B1F"}},
			"outline": {"dark": {"color": "#888888"}, "default": {"color": "#888888"}, "light": {"color": "#79747E"}},
			"outline_variant": {"dark": {"color": "#444444"}, "default": {"color": "#444444"}, "light": {"color": "#CAC4D0"}}
		},
		"mode": "dark",
		"is_dark_mode": true
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
