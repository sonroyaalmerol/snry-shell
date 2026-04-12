package controlpanel

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/controlsocket"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
	"github.com/sonroyaalmerol/snry-shell/internal/theme"
)

type wallpaperConfigProvider struct {
	baseShellProvider
	preview *gtk.Picture
}

func newWallpaperConfigProvider(cfg *settings.Config) *wallpaperConfigProvider {
	return &wallpaperConfigProvider{baseShellProvider: baseShellProvider{cfg: cfg}}
}

func (w *wallpaperConfigProvider) Name() string { return "Wallpaper" }
func (w *wallpaperConfigProvider) Icon() string { return "image" }

func (w *wallpaperConfigProvider) BuildWidget() gtk.Widgetter {
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("popup-scroll")

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("settings-stack")

	// ── Preview ──────────────────────────────────────────────────────────────

	w.preview = gtk.NewPicture()
	w.preview.SetContentFit(gtk.ContentFitCover)
	w.preview.SetCanShrink(true)
	w.preview.AddCSSClass("wallpaper-preview")

	// Show the last processed wallpaper as the preview. Try multiple
	// sources since the control panel is a separate process and the
	// persistent store may not be populated yet.
	if processed := theme.GetLastWallpaper(); processed != "" {
		w.preview.SetFilename(processed)
	} else if _, err := os.Stat(theme.ProcessedWallpaperPath()); err == nil {
		w.preview.SetFilename(theme.ProcessedWallpaperPath())
	} else if src := theme.GetWallpaperSource(); src != "" {
		w.preview.SetFilename(src)
	}

	previewCard := gtk.NewBox(gtk.OrientationVertical, 0)
	previewCard.AddCSSClass("m3-card")
	previewCard.SetSizeRequest(-1, 200)
	previewCard.Append(w.preview)

	previewSection := gtk.NewBox(gtk.OrientationVertical, 12)
	previewSection.AddCSSClass("settings-section")
	previewLbl := gtk.NewLabel("Current Wallpaper")
	previewLbl.AddCSSClass("settings-subtitle")
	previewLbl.SetHAlign(gtk.AlignStart)
	previewSection.Append(previewLbl)
	previewSection.Append(previewCard)
	box.Append(previewSection)

	// ── File selection ───────────────────────────────────────────────────────

	sourcePath := theme.GetWallpaperSource()

	pathField := gtkutil.NewM3OutlinedTextField()
	pathField.SetText(sourcePath)
	pathField.SetHExpand(true)
	pathField.Entry().SetEditable(false)

	browseBtn := gtkutil.M3IconButton("folder_open", "settings-btn")
	browseBtn.SetTooltipText("Browse for wallpaper")
	browseBtn.ConnectClicked(func() {
		chooser := gtk.NewFileChooserNative("Select Wallpaper", nil, gtk.FileChooserActionOpen, "Select", "Cancel")

		filter := gtk.NewFileFilter()
		filter.SetName("Images")
		filter.AddMIMEType("image/jpeg")
		filter.AddMIMEType("image/png")
		filter.AddMIMEType("image/webp")
		chooser.AddFilter(filter)

		chooser.ConnectResponse(func(res int) {
			if res == int(gtk.ResponseAccept) {
				if file := chooser.File(); file != nil {
					path := file.Path()
					pathField.SetText(path)
					// Show source immediately in preview while the shell processes it.
					w.preview.SetFilename(path)
					w.applyWallpaper(path)
				}
			}
			chooser.Destroy()
		})
		chooser.Show()
	})

	selRow := gtk.NewBox(gtk.OrientationHorizontal, 16)
	selRow.AddCSSClass("m3-switch-row")
	selRow.Append(pathField)
	selRow.Append(browseBtn)

	selCard := gtk.NewBox(gtk.OrientationVertical, 0)
	selCard.AddCSSClass("system-controls")
	selCard.Append(selRow)

	selSection := gtk.NewBox(gtk.OrientationVertical, 12)
	selSection.AddCSSClass("settings-section")
	selSection.SetMarginTop(24)
	selLbl := gtk.NewLabel("Selection")
	selLbl.AddCSSClass("settings-subtitle")
	selLbl.SetHAlign(gtk.AlignStart)
	selSection.Append(selLbl)
	selSection.Append(selCard)
	box.Append(selSection)

	// ── Display options ──────────────────────────────────────────────────────

	fitRow := gtkutil.DropdownRow(
		"Fit Mode", "How the image fills the screen",
		[]string{"cover", "contain", "fill", "scale-down"},
		w.cfg.WallpaperFit,
		func(v string) {
			w.cfg.WallpaperFit = v
			w.Save()
		},
	)

	box.Append(gtkutil.SettingsSection("Display", fitRow))

	// ── Processing options ───────────────────────────────────────────────────

	blurRow := gtkutil.SpinRow(
		"Blur", "Gaussian blur radius applied to the wallpaper (0 = off, 1–50)",
		0, 50, w.cfg.WallpaperBlur,
		func(v int) {
			w.cfg.WallpaperBlur = v
			w.Save()
		},
	)

	brightnessRow := gtkutil.SpinRow(
		"Brightness", "Brightness multiplier: 100 = original, <100 = darker, >100 = brighter",
		0, 200, w.cfg.WallpaperBrightness,
		func(v int) {
			w.cfg.WallpaperBrightness = v
			w.Save()
		},
	)

	grayscaleRow, _ := gtkutil.SwitchRowFull(
		"Grayscale", "Convert the wallpaper to black & white",
		w.cfg.WallpaperGrayscale,
		func(active bool) {
			w.cfg.WallpaperGrayscale = active
			w.Save()
		},
	)

	box.Append(gtkutil.SettingsSection("Processing", blurRow, brightnessRow, grayscaleRow))

	scroll.SetChild(box)
	return scroll
}

// applyWallpaper tells the running shell to process and apply a new wallpaper.
func (w *wallpaperConfigProvider) applyWallpaper(path string) {
	w.cfg.WallpaperSource = path

	conn, err := net.Dial("unix", controlsocket.DefaultPath)
	if err != nil {
		log.Printf("[CONTROLPANEL] connect to shell socket: %v", err)
		return
	}
	defer conn.Close()
	fmt.Fprintf(conn, "set-wallpaper:%s", path)
}
