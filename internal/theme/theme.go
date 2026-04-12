// Package theme provides dynamic color scheme generation from wallpaper images.
// It extracts dominant colors and generates Material Design 3 color tokens.
package theme

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sonroyaalmerol/snry-shell/internal/fileutil"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
	"github.com/sonroyaalmerol/snry-shell/internal/store"
)

const (
	storeKeyWallpaper = "theme.wallpaper"
)

// ColorScheme holds all Material Design 3 color tokens
type ColorScheme struct {
	Primary                 string
	OnPrimary               string
	PrimaryContainer        string
	OnPrimaryContainer      string
	Secondary               string
	OnSecondary             string
	SecondaryContainer      string
	OnSecondaryContainer    string
	Tertiary                string
	OnTertiary              string
	TertiaryContainer       string
	OnTertiaryContainer     string
	Error                   string
	OnError                 string
	ErrorContainer          string
	OnErrorContainer        string
	Surface                 string
	SurfaceDim              string
	SurfaceBright           string
	SurfaceContainer        string
	SurfaceContainerLow     string
	SurfaceContainerHigh    string
	SurfaceContainerHighest string
	OnSurface               string
	OnSurfaceVariant        string
	Background              string
	OnBackground            string
	Outline                 string
	OutlineVariant          string
	Subtext                 string
}

// Generator handles wallpaper color extraction and theme generation
type Generator struct {
	wallpaperPath string
	blurStrength  int
	cacheDir      string
}

// New creates a new theme generator
func New() *Generator {
	return &Generator{
		cacheDir:     filepath.Join(fileutil.CacheDir(), "snry-shell"),
		blurStrength: 20,
	}
}

// GetLastWallpaper returns the path of the last processed wallpaper.
func GetLastWallpaper() string {
	return store.LookupOr(storeKeyWallpaper, "")
}

// GetWallpaperSource returns the original user-selected wallpaper path.
// This is the path shown in the control panel file picker.
func GetWallpaperSource() string {
	if cfg, err := settings.Load(); err == nil && cfg.WallpaperSource != "" {
		return cfg.WallpaperSource
	}
	return ""
}

// SetBlurStrength updates blur strength and regenerates theme
func (g *Generator) SetBlurStrength(v int) error {
	g.blurStrength = v
	return g.Generate()
}

// SetWallpaper sets the current wallpaper path and regenerates theme
func (g *Generator) SetWallpaper(path string) error {
	if path == g.wallpaperPath {
		return nil
	}
	g.wallpaperPath = path

	// Save to persistent store
	if err := store.Set(storeKeyWallpaper, path); err != nil {
		log.Printf("[theme] Failed to save wallpaper path: %v", err)
	}

	return g.Generate()
}

// Generate extracts colors from wallpaper and writes theme.css
func (g *Generator) Generate() error {
	if g.wallpaperPath == "" {
		return nil
	}

	colors, err := g.extractColors()
	if err != nil {
		return fmt.Errorf("extract colors: %w", err)
	}

	scheme := g.generateScheme(colors)
	return g.writeThemeCSS(scheme)
}

// extractColors extracts dominant colors from the wallpaper image
func (g *Generator) extractColors() ([]color.Color, error) {
	file, err := os.Open(g.wallpaperPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	// Sample pixels at regular intervals for performance
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Collect colors using a grid sampling approach
	colorMap := make(map[uint32]int)
	step := 10 // Sample every 10th pixel

	for y := 0; y < height; y += step {
		for x := 0; x < width; x += step {
			c := img.At(bounds.Min.X+x, bounds.Min.Y+y)
			r, g, b, _ := c.RGBA()
			// Quantize to reduce color space (6 bits per channel)
			key := (uint32(r>>10) << 12) | (uint32(g>>10) << 6) | uint32(b>>10)
			colorMap[key]++
		}
	}

	// Find most frequent colors
	type colorCount struct {
		key   uint32
		count int
	}
	var counts []colorCount
	for k, v := range colorMap {
		counts = append(counts, colorCount{k, v})
	}

	sort.Slice(counts, func(i, j int) bool {
		return counts[i].count > counts[j].count
	})

	// Extract top colors and convert back to RGB
	var colors []color.Color
	for i := 0; i < min(5, len(counts)); i++ {
		r := (counts[i].key >> 12) & 0x3F
		g := (counts[i].key >> 6) & 0x3F
		b := counts[i].key & 0x3F
		// Scale back to 16-bit
		c := color.RGBA64{
			R: uint16(r << 10),
			G: uint16(g << 10),
			B: uint16(b << 10),
			A: 0xFFFF,
		}
		colors = append(colors, c)
	}

	if len(colors) == 0 {
		// Fallback colors
		colors = []color.Color{
			color.RGBA{187, 134, 252, 255}, // Purple-ish
			color.RGBA{3, 218, 198, 255},   // Teal
			color.RGBA{207, 102, 121, 255}, // Pink
		}
	}

	return colors, nil
}

// generateScheme creates a Material Design 3 color scheme from extracted colors
func (g *Generator) generateScheme(dominantColors []color.Color) *ColorScheme {
	if len(dominantColors) == 0 {
		return defaultScheme()
	}

	// Use the most dominant color as the primary source
	primaryColor := dominantColors[0]

	// Convert to HSL for easier manipulation
	primaryH, primaryS, _ := rgbToHsl(primaryColor)

	// Generate primary palette
	primary := hslToHex(primaryH, primaryS, 0.65)
	onPrimary := contrastColor(primaryH, primaryS, 0.65)
	primaryContainer := hslToHex(primaryH, primaryS*0.8, 0.85)
	onPrimaryContainer := hslToHex(primaryH, primaryS, 0.15)

	// Generate secondary (analogous to primary)
	secondaryH := math.Mod(primaryH+30, 360)
	secondary := hslToHex(secondaryH, 0.6, 0.60)
	onSecondary := contrastColor(secondaryH, 0.6, 0.60)
	secondaryContainer := hslToHex(secondaryH, 0.5, 0.88)
	onSecondaryContainer := hslToHex(secondaryH, 0.7, 0.15)

	// Generate tertiary (complementary-ish)
	tertiaryH := math.Mod(primaryH+60, 360)
	tertiary := hslToHex(tertiaryH, 0.55, 0.65)
	onTertiary := contrastColor(tertiaryH, 0.55, 0.65)
	tertiaryContainer := hslToHex(tertiaryH, 0.5, 0.88)
	onTertiaryContainer := hslToHex(tertiaryH, 0.7, 0.15)

	// Generate surface colors based on luminance of primary
	surfaceL := 0.12 // Dark theme base
	if len(dominantColors) > 1 {
		// Adjust surface based on wallpaper brightness
		_, _, l := rgbToHsl(dominantColors[1])
		if l > 0.5 {
			surfaceL = 0.15 + (l-0.5)*0.1
		}
	}

	surface := hslToHex(primaryH, 0.05, surfaceL)
	surfaceDim := hslToHex(primaryH, 0.05, surfaceL-0.02)
	surfaceBright := hslToHex(primaryH, 0.08, surfaceL+0.08)
	surfaceContainer := hslToHex(primaryH, 0.04, surfaceL+0.02)
	surfaceContainerLow := hslToHex(primaryH, 0.04, surfaceL-0.01)
	surfaceContainerHigh := hslToHex(primaryH, 0.06, surfaceL+0.05)
	surfaceContainerHighest := hslToHex(primaryH, 0.08, surfaceL+0.10)
	onSurface := hslToHex(primaryH, 0.05, 0.90)
	onSurfaceVariant := hslToHex(primaryH, 0.05, 0.70)
	background := hslToHex(primaryH, 0.08, surfaceL-0.03)
	onBackground := onSurface

	outline := hslToHex(primaryH, 0.10, 0.45)
	outlineVariant := hslToHex(primaryH, 0.08, 0.35)
	subtext := onSurfaceVariant

	return &ColorScheme{
		Primary:                 primary,
		OnPrimary:               onPrimary,
		PrimaryContainer:        primaryContainer,
		OnPrimaryContainer:      onPrimaryContainer,
		Secondary:               secondary,
		OnSecondary:             onSecondary,
		SecondaryContainer:      secondaryContainer,
		OnSecondaryContainer:    onSecondaryContainer,
		Tertiary:                tertiary,
		OnTertiary:              onTertiary,
		TertiaryContainer:       tertiaryContainer,
		OnTertiaryContainer:     onTertiaryContainer,
		Error:                   "#CF6679",
		OnError:                 "#000000",
		ErrorContainer:          "#CF6679",
		OnErrorContainer:        "#000000",
		Surface:                 surface,
		SurfaceDim:              surfaceDim,
		SurfaceBright:           surfaceBright,
		SurfaceContainer:        surfaceContainer,
		SurfaceContainerLow:     surfaceContainerLow,
		SurfaceContainerHigh:    surfaceContainerHigh,
		SurfaceContainerHighest: surfaceContainerHighest,
		OnSurface:               onSurface,
		OnSurfaceVariant:        onSurfaceVariant,
		Background:              background,
		OnBackground:            onBackground,
		Outline:                 outline,
		OutlineVariant:          outlineVariant,
		Subtext:                 subtext,
	}
}

// writeThemeCSS writes the color scheme to theme.css
func (g *Generator) writeThemeCSS(scheme *ColorScheme) error {
	if err := os.MkdirAll(g.cacheDir, 0755); err != nil {
		return err
	}

	css := g.generateCSS(scheme)
	themePath := filepath.Join(g.cacheDir, "theme.css")

	if err := os.WriteFile(themePath, []byte(css), 0644); err != nil {
		return err
	}

	log.Printf("[theme] Generated theme.css from wallpaper: %s", g.wallpaperPath)
	return nil
}

// generateCSS creates the CSS content from a color scheme
func (g *Generator) generateCSS(scheme *ColorScheme) string {
	var sb strings.Builder
	sb.WriteString("/* Auto-generated theme from wallpaper */\n")
	sb.WriteString("/* This file is overwritten when wallpaper changes */\n\n")

	sb.WriteString(fmt.Sprintf("* {\n    --blur-strength: %dpx;\n}\n\n", g.blurStrength))

	writeColor(&sb, "col_primary", scheme.Primary)
	writeColor(&sb, "col_on_primary", scheme.OnPrimary)
	writeColor(&sb, "col_primary_container", scheme.PrimaryContainer)
	writeColor(&sb, "col_on_primary_container", scheme.OnPrimaryContainer)
	writeColor(&sb, "col_secondary", scheme.Secondary)
	writeColor(&sb, "col_on_secondary", scheme.OnSecondary)
	writeColor(&sb, "col_secondary_container", scheme.SecondaryContainer)
	writeColor(&sb, "col_on_secondary_container", scheme.OnSecondaryContainer)
	writeColor(&sb, "col_tertiary", scheme.Tertiary)
	writeColor(&sb, "col_on_tertiary", scheme.OnTertiary)
	writeColor(&sb, "col_tertiary_container", scheme.TertiaryContainer)
	writeColor(&sb, "col_on_tertiary_container", scheme.OnTertiaryContainer)
	writeColor(&sb, "col_error", scheme.Error)
	writeColor(&sb, "col_on_error", scheme.OnError)
	writeColor(&sb, "col_error_container", scheme.ErrorContainer)
	writeColor(&sb, "col_on_error_container", scheme.OnErrorContainer)
	writeColor(&sb, "col_surface", scheme.Surface)
	writeColor(&sb, "col_surface_dim", scheme.SurfaceDim)
	writeColor(&sb, "col_surface_bright", scheme.SurfaceBright)
	writeColor(&sb, "col_surface_container", scheme.SurfaceContainer)
	writeColor(&sb, "col_surface_container_low", scheme.SurfaceContainerLow)
	writeColor(&sb, "col_surface_container_high", scheme.SurfaceContainerHigh)
	writeColor(&sb, "col_surface_container_highest", scheme.SurfaceContainerHighest)
	writeColor(&sb, "col_on_surface", scheme.OnSurface)
	writeColor(&sb, "col_on_surface_variant", scheme.OnSurfaceVariant)
	writeColor(&sb, "col_background", scheme.Background)
	writeColor(&sb, "col_on_background", scheme.OnBackground)
	writeColor(&sb, "col_outline", scheme.Outline)
	writeColor(&sb, "col_outline_variant", scheme.OutlineVariant)
	writeColor(&sb, "col_subtext", scheme.Subtext)

	return sb.String()
}

func writeColor(sb *strings.Builder, name, value string) {
	sb.WriteString(fmt.Sprintf("@define-color %s %s;\n", name, value))
}

// Helper functions

func rgbToHsl(c color.Color) (h, s, l float64) {
	r, g, b, _ := c.RGBA()

	// Convert to 0-1 range
	rf := float64(r) / 65535.0
	gf := float64(g) / 65535.0
	bf := float64(b) / 65535.0

	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	l = (max + min) / 2.0

	if max == min {
		h = 0
		s = 0
	} else {
		d := max - min
		if l > 0.5 {
			s = d / (2.0 - max - min)
		} else {
			s = d / (max + min)
		}

		switch max {
		case rf:
			h = math.Mod((gf-bf)/d, 6.0)
		case gf:
			h = (bf-rf)/d + 2.0
		case bf:
			h = (rf-gf)/d + 4.0
		}
		h *= 60.0
		if h < 0 {
			h += 360.0
		}
	}

	return h, s, l
}

func hslToHex(h, s, l float64) string {
	c := (1.0 - math.Abs(2.0*l-1.0)) * s
	x := c * (1.0 - math.Abs(math.Mod(h/60.0, 2.0)-1.0))
	m := l - c/2.0

	var r, g, b float64

	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	r = math.Round((r + m) * 255.0)
	g = math.Round((g + m) * 255.0)
	b = math.Round((b + m) * 255.0)

	r = math.Max(0, math.Min(255, r))
	g = math.Max(0, math.Min(255, g))
	b = math.Max(0, math.Min(255, b))

	return fmt.Sprintf("#%02X%02X%02X", int(r), int(g), int(b))
}

func contrastColor(h, s, l float64) string {
	// Return black or white based on luminance for good contrast
	if l > 0.5 {
		return "#000000"
	}
	return "#FFFFFF"
}

func defaultScheme() *ColorScheme {
	return &ColorScheme{
		Primary:                 "#BB86FC",
		OnPrimary:               "#000000",
		PrimaryContainer:        "#BB86FC",
		OnPrimaryContainer:      "#000000",
		Secondary:               "#03DAC6",
		OnSecondary:             "#000000",
		SecondaryContainer:      "#03DAC6",
		OnSecondaryContainer:    "#000000",
		Tertiary:                "#CF6679",
		OnTertiary:              "#000000",
		TertiaryContainer:       "#CF6679",
		OnTertiaryContainer:     "#000000",
		Error:                   "#CF6679",
		OnError:                 "#000000",
		ErrorContainer:          "#CF6679",
		OnErrorContainer:        "#000000",
		Surface:                 "#1E1E2E",
		SurfaceDim:              "#181825",
		SurfaceBright:           "#313244",
		SurfaceContainer:        "#1E1E2E",
		SurfaceContainerLow:     "#181825",
		SurfaceContainerHigh:    "#313244",
		SurfaceContainerHighest: "#45475A",
		OnSurface:               "#CDD6F4",
		OnSurfaceVariant:        "#A6ADC8",
		Background:              "#11111B",
		OnBackground:            "#CDD6F4",
		Outline:                 "#585B70",
		OutlineVariant:          "#45475A",
		Subtext:                 "#A6ADC8",
	}
}

// ThemePath returns the path to the generated theme.css
func ThemePath() string {
	return filepath.Join(fileutil.CacheDir(), "snry-shell", "theme.css")
}
