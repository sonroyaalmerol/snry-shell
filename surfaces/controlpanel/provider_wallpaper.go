package controlpanel

import (
	"fmt"
	"log"
	"net"

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

	// 1. Current Wallpaper Preview
	previewBox := gtk.NewBox(gtk.OrientationVertical, 12)
	previewBox.AddCSSClass("settings-section")

	subtitle := gtk.NewLabel("Current Wallpaper")
	subtitle.AddCSSClass("settings-subtitle")
	subtitle.SetHAlign(gtk.AlignStart)
	previewBox.Append(subtitle)

	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("m3-card")
	card.SetSizeRequest(-1, 200)

	w.preview = gtk.NewPicture()
	w.preview.SetContentFit(gtk.ContentFitCover)
	w.preview.SetCanShrink(true)
	w.preview.AddCSSClass("wallpaper-preview")
	
	wallpaperPath := theme.GetLastWallpaper()
	if wallpaperPath != "" {
		w.preview.SetFilename(wallpaperPath)
	}
	
	card.Append(w.preview)
	previewBox.Append(card)
	box.Append(previewBox)

	// 2. Select Wallpaper
	selectionBox := gtk.NewBox(gtk.OrientationVertical, 12)
	selectionBox.AddCSSClass("settings-section")
	selectionBox.SetMarginTop(24)

	selSubtitle := gtk.NewLabel("Selection")
	selSubtitle.AddCSSClass("settings-subtitle")
	selSubtitle.SetHAlign(gtk.AlignStart)
	selectionBox.Append(selSubtitle)

	selCard := gtk.NewBox(gtk.OrientationVertical, 0)
	selCard.AddCSSClass("system-controls")

	row := gtk.NewBox(gtk.OrientationHorizontal, 16)
	row.AddCSSClass("m3-switch-row")

	pathField := gtkutil.NewM3OutlinedTextField()
	pathField.SetText(wallpaperPath)
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
				file := chooser.File()
				if file != nil {
					path := file.Path()
					pathField.SetText(path)
					w.updateWallpaper(path)
				}
			}
			chooser.Destroy()
		})
		chooser.Show()
	})

	row.Append(pathField)
	row.Append(browseBtn)
	selCard.Append(row)
	selectionBox.Append(selCard)
	box.Append(selectionBox)

	// 3. Wallpaper Daemon
	daemonBox := gtk.NewBox(gtk.OrientationVertical, 12)
	daemonBox.AddCSSClass("settings-section")
	daemonBox.SetMarginTop(24)

	daemonSubtitle := gtk.NewLabel("Wallpaper Daemon")
	daemonSubtitle.AddCSSClass("settings-subtitle")
	daemonSubtitle.SetHAlign(gtk.AlignStart)
	daemonBox.Append(daemonSubtitle)

	daemonRow := gtkutil.DropdownRow("Background Daemon", "Tool used to set desktop wallpaper",
		[]string{"auto", "hyprpaper", "swww", "swaybg", "wbg"}, w.cfg.WallpaperDaemon,
		func(v string) {
			w.cfg.WallpaperDaemon = v
			w.Save()
		})

	daemonBox.Append(gtkutil.SettingsSection("", daemonRow))
	box.Append(daemonBox)

	scroll.SetChild(box)
	return scroll
}

func (w *wallpaperConfigProvider) updateWallpaper(path string) {
	// 1. Update preview
	w.preview.SetFilename(path)

	// 2. Notify shell to apply and regenerate theme
	conn, err := net.Dial("unix", controlsocket.DefaultPath)
	if err != nil {
		log.Printf("[CONTROLPANEL] failed to connect to shell socket: %v", err)
		return
	}
	defer conn.Close()
	
	cmd := fmt.Sprintf("set-wallpaper:%s", path)
	conn.Write([]byte(cmd))
}
