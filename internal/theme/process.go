package theme

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"os"
	"path/filepath"

	_ "golang.org/x/image/webp"

	"github.com/sonroyaalmerol/snry-shell/internal/fileutil"
)

const maxWallpaperDim = 1920

// ProcessConfig holds image post-processing parameters for the wallpaper.
type ProcessConfig struct {
	Blur       int // 0–50  (0 = no blur)
	Brightness int // 0–200 (100 = no change, <100 = darker, >100 = brighter)
	Grayscale  bool
}

// ProcessedWallpaperPath returns the fixed path where the processed
// wallpaper is written. Always a PNG.
func ProcessedWallpaperPath() string {
	return filepath.Join(fileutil.CacheDir(), "snry-shell", "wallpaper.png")
}

// ProcessWallpaper loads the image at src, applies the requested
// adjustments (grayscale → brightness → blur), writes the result to the
// fixed processed path, and returns that path. If all processing params
// are identity (blur=0, brightness=100, grayscale=false), the source is
// copied directly without decoding/encoding.
func ProcessWallpaper(src string, cfg ProcessConfig) (string, error) {
	dst := ProcessedWallpaperPath()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}

	// Fast path: no processing needed, but still downsample to save GPU memory.
	if cfg.Blur == 0 && cfg.Brightness == 100 && !cfg.Grayscale {
		if err := downsampleAndSave(src, dst); err != nil {
			// Fallback: if downsampling fails, just copy.
			data, err := os.ReadFile(src)
			if err != nil {
				return "", fmt.Errorf("read wallpaper source: %w", err)
			}
			if err := os.WriteFile(dst, data, 0644); err != nil {
				return "", fmt.Errorf("write processed wallpaper: %w", err)
			}
		}
		return dst, nil
	}

	// Slow path: decode, process, re-encode.
	f, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("open wallpaper source: %w", err)
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return "", fmt.Errorf("decode wallpaper: %w", err)
	}

	rgba := toNRGBA(img)

	if cfg.Grayscale {
		applyGrayscale(rgba)
	}

	if cfg.Brightness != 100 {
		applyBrightness(rgba, cfg.Brightness)
	}

	if cfg.Blur > 0 {
		// Three passes of separable box blur approximates a Gaussian.
		// Preallocate two scratch buffers and ping-pong between them so
		// each pass reuses memory instead of allocating two fresh images.
		b := rgba.Bounds()
		horiz := image.NewNRGBA(b) // horizontal-pass scratch
		tmp := image.NewNRGBA(b)   // output ping-pong buffer
		for range 3 {
			separableBoxBlurInto(rgba, horiz, tmp, cfg.Blur)
			rgba, tmp = tmp, rgba // result is now rgba; old rgba becomes next output
		}
	}

	// Downsample to reduce GPU memory usage.
	rgba = downsample(rgba, maxWallpaperDim)

	out, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("create processed wallpaper: %w", err)
	}
	defer out.Close()
	if err := png.Encode(out, rgba); err != nil {
		return "", fmt.Errorf("encode wallpaper: %w", err)
	}
	return dst, nil
}

// toNRGBA converts any image.Image to *image.NRGBA for in-place pixel edits.
func toNRGBA(src image.Image) *image.NRGBA {
	if nrgba, ok := src.(*image.NRGBA); ok {
		return nrgba
	}
	b := src.Bounds()
	dst := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}
	return dst
}

// applyGrayscale converts an NRGBA image to grayscale in-place.
func applyGrayscale(img *image.NRGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			// Luminance-weighted average (BT.601).
			lum := uint8((uint32(c.R)*299 + uint32(c.G)*587 + uint32(c.B)*114) / 1000)
			img.SetNRGBA(x, y, color.NRGBA{R: lum, G: lum, B: lum, A: c.A})
		}
	}
}

// applyBrightness scales each channel by brightness/100, clamping to 0–255.
func applyBrightness(img *image.NRGBA, brightness int) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			img.SetNRGBA(x, y, color.NRGBA{
				R: clampU8(int(c.R) * brightness / 100),
				G: clampU8(int(c.G) * brightness / 100),
				B: clampU8(int(c.B) * brightness / 100),
				A: c.A,
			})
		}
	}
}

// separableBoxBlurInto performs a single horizontal+vertical box blur pass
// using sliding-window running sums — O(w·h) regardless of radius.
// It writes the result into dst, using horiz as a scratch buffer for the
// horizontal pass. Both horiz and dst must have the same bounds as src.
func separableBoxBlurInto(src, horiz, dst *image.NRGBA, radius int) {
	b := src.Bounds()
	w := b.Dx()
	h := b.Dy()
	ox := b.Min.X
	oy := b.Min.Y

	// Horizontal pass: src → horiz
	for y := range h {
		var sumR, sumG, sumB int
		cnt := 0
		for dx := 0; dx <= radius && dx < w; dx++ {
			c := src.NRGBAAt(ox+dx, oy+y)
			sumR += int(c.R)
			sumG += int(c.G)
			sumB += int(c.B)
			cnt++
		}
		horiz.SetNRGBA(ox, oy+y, color.NRGBA{R: uint8(sumR / cnt), G: uint8(sumG / cnt), B: uint8(sumB / cnt), A: 255})

		for x := 1; x < w; x++ {
			if outX := x - radius - 1; outX >= 0 {
				c := src.NRGBAAt(ox+outX, oy+y)
				sumR -= int(c.R)
				sumG -= int(c.G)
				sumB -= int(c.B)
				cnt--
			}
			if inX := x + radius; inX < w {
				c := src.NRGBAAt(ox+inX, oy+y)
				sumR += int(c.R)
				sumG += int(c.G)
				sumB += int(c.B)
				cnt++
			}
			horiz.SetNRGBA(ox+x, oy+y, color.NRGBA{R: uint8(sumR / cnt), G: uint8(sumG / cnt), B: uint8(sumB / cnt), A: 255})
		}
	}

	// Vertical pass: horiz → dst
	for x := range w {
		var sumR, sumG, sumB int
		cnt := 0
		for dy := 0; dy <= radius && dy < h; dy++ {
			c := horiz.NRGBAAt(ox+x, oy+dy)
			sumR += int(c.R)
			sumG += int(c.G)
			sumB += int(c.B)
			cnt++
		}
		dst.SetNRGBA(ox+x, oy, color.NRGBA{R: uint8(sumR / cnt), G: uint8(sumG / cnt), B: uint8(sumB / cnt), A: 255})

		for y := 1; y < h; y++ {
			if outY := y - radius - 1; outY >= 0 {
				c := horiz.NRGBAAt(ox+x, oy+outY)
				sumR -= int(c.R)
				sumG -= int(c.G)
				sumB -= int(c.B)
				cnt--
			}
			if inY := y + radius; inY < h {
				c := horiz.NRGBAAt(ox+x, oy+inY)
				sumR += int(c.R)
				sumG += int(c.G)
				sumB += int(c.B)
				cnt++
			}
			dst.SetNRGBA(ox+x, oy+y, color.NRGBA{R: uint8(sumR / cnt), G: uint8(sumG / cnt), B: uint8(sumB / cnt), A: 255})
		}
	}
}

func clampU8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// downsample scales img down so its largest dimension is at most maxDim.
// If the image is already within bounds, it is returned unchanged.
func downsample(img *image.NRGBA, maxDim int) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxDim && h <= maxDim {
		return img
	}

	scale := float64(maxDim) / float64(max(w, h))
	nw := int(float64(w) * scale)
	nh := int(float64(h) * scale)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}

	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	// Simple area-average downscale.
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			// Map destination pixel to source region.
			sx0 := b.Min.X + x*w/nw
			sy0 := b.Min.Y + y*h/nh
			sx1 := b.Min.X + (x+1)*w/nw
			sy1 := b.Min.Y + (y+1)*h/nh

			var r, g, b, a, cnt uint32
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					c := img.NRGBAAt(sx, sy)
					r += uint32(c.R)
					g += uint32(c.G)
					b += uint32(c.B)
					a += uint32(c.A)
					cnt++
				}
			}
			if cnt > 0 {
				dst.SetNRGBA(x, y, color.NRGBA{
					R: uint8(r / cnt),
					G: uint8(g / cnt),
					B: uint8(b / cnt),
					A: uint8(a / cnt),
				})
			}
		}
	}
	return dst
}

// downsampleAndSave decodes, downsamples, and saves as PNG.
func downsampleAndSave(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	nrgba := toNRGBA(img)
	nrgba = downsample(nrgba, maxWallpaperDim)

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	return png.Encode(out, nrgba)
}
