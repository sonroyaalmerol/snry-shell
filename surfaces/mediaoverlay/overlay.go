// Package mediaoverlay provides a full-screen media player controls overlay.
package mediaoverlay

import (
	"context"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/services/mpris"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

// MediaOverlay is a floating music player popup.
type MediaOverlay struct {
	win          *gtk.ApplicationWindow
	bus          *bus.Bus
	mprisSvc     *mpris.Service
	player       state.MediaPlayer
	art          *gtk.Picture
	title        *gtk.Label
	artist       *gtk.Label
	scale        *gtk.Scale
	posLabel     *gtk.Label
	durLabel     *gtk.Label
	playBtn      *gtk.Button
	prevBtn      *gtk.Button
	nextBtn      *gtk.Button
	tickerCtx    context.CancelFunc
	changeHandle glib.SignalHandle
}

func New(app *gtk.Application, b *bus.Bus, mprisSvc *mpris.Service) *MediaOverlay {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-media-overlay",
		Layer:         layershell.LayerTop,
		Anchors:       map[layershell.Edge]bool{layershell.EdgeTop: true, layershell.EdgeRight: true},
		Margins:       map[layershell.Edge]int{layershell.EdgeTop: 44, layershell.EdgeRight: 8},
		KeyboardMode:  layershell.KeyboardModeNone,
		ExclusiveZone: -1,
		Namespace:     "snry-media-overlay",
	})

	mo := &MediaOverlay{win: win, bus: b, mprisSvc: mprisSvc}
	mo.build()

	// Toggle on SystemControls "toggle-media-overlay".
	surfaceutil.AddToggleOn(b, win, "toggle-media-overlay")

	win.SetVisible(false)
	return mo
}

func (mo *MediaOverlay) build() {
	root := gtk.NewBox(gtk.OrientationVertical, 8)
	root.AddCSSClass("media-overlay")

	// Top row: album art + text.
	top := gtk.NewBox(gtk.OrientationHorizontal, 12)
	top.SetHAlign(gtk.AlignFill)

	mo.art = gtk.NewPicture()
	mo.art.AddCSSClass("media-overlay-art")

	info := gtk.NewBox(gtk.OrientationVertical, 2)
	mo.title = gtk.NewLabel("")
	mo.title.AddCSSClass("media-overlay-title")
	mo.title.SetHAlign(gtk.AlignStart)
	mo.artist = gtk.NewLabel("")
	mo.artist.AddCSSClass("media-overlay-artist")
	mo.artist.SetHAlign(gtk.AlignStart)

	info.Append(mo.title)
	info.Append(mo.artist)
	top.Append(mo.art)
	top.Append(info)

	// Progress row.
	progressRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	mo.posLabel = gtk.NewLabel("0:00")
	mo.posLabel.AddCSSClass("media-overlay-position")
	mo.posLabel.SetHAlign(gtk.AlignStart)

	mo.scale = gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 1, 0.001)
	mo.scale.AddCSSClass("media-progress")
	mo.scale.SetDrawValue(false)
	mo.scale.SetHExpand(true)
	mo.scale.SetSensitive(true)
	mo.changeHandle = mo.scale.ConnectChangeValue(func(_ gtk.ScrollType, value float64) bool {
		go mo.mprisSvc.SeekTo(mo.player.PlayerName, value*mo.player.Duration)
		return false
	})

	mo.durLabel = gtk.NewLabel("0:00")
	mo.durLabel.AddCSSClass("media-overlay-position")
	mo.durLabel.SetHAlign(gtk.AlignEnd)

	progressRow.Append(mo.posLabel)
	progressRow.Append(mo.scale)
	progressRow.Append(mo.durLabel)

	// Controls row.
	controls := gtk.NewBox(gtk.OrientationHorizontal, 0)
	controls.SetHAlign(gtk.AlignCenter)
	controls.SetMarginTop(4)

	mo.prevBtn = gtkutil.MaterialButton("skip_previous")
	mo.prevBtn.AddCSSClass("media-btn")
	mo.prevBtn.ConnectClicked(func() {
		go mo.mprisSvc.Previous(mo.player.PlayerName)
	})

	mo.playBtn = gtkutil.MaterialButtonWithClass("play_arrow", "media-btn", "play-pause")
	mo.playBtn.ConnectClicked(func() {
		go mo.mprisSvc.PlayPause(mo.player.PlayerName)
	})

	mo.nextBtn = gtkutil.MaterialButton("skip_next")
	mo.nextBtn.AddCSSClass("media-btn")
	mo.nextBtn.ConnectClicked(func() {
		go mo.mprisSvc.Next(mo.player.PlayerName)
	})

	controls.Append(mo.prevBtn)
	controls.Append(mo.playBtn)
	controls.Append(mo.nextBtn)

	root.Append(top)
	root.Append(progressRow)
	root.Append(controls)
	mo.win.SetChild(root)

	// Subscribe to media state changes.
	mo.bus.Subscribe(bus.TopicMedia, func(e bus.Event) {
		mp := e.Data.(state.MediaPlayer)
		glib.IdleAdd(func() { mo.updatePlayer(mp) })
	})

	// Subscribe to position ticks.
	mo.bus.Subscribe(bus.TopicMediaTick, func(e bus.Event) {
		tick := e.Data.(state.MediaTick)
		glib.IdleAdd(func() { mo.updatePosition(tick.Position, tick.Duration) })
	})
}

func (mo *MediaOverlay) updatePlayer(mp state.MediaPlayer) {
	mo.player = mp
	if mp.ArtPath != "" {
		mo.art.SetFilename(mp.ArtPath)
	}
	mo.title.SetText(mp.Title)
	mo.artist.SetText(mp.Artist)
	mo.durLabel.SetText(surfaceutil.FormatTime(mp.Duration))

	if mp.Duration > 0 {
		mo.scale.SetRange(0, mp.Duration)
	}
	mo.scale.HandlerBlock(mo.changeHandle)
	mo.scale.SetValue(mp.Position)
	mo.scale.HandlerUnblock(mo.changeHandle)

	// Update play button icon.
	icon := "play_arrow"
	if mp.Playing {
		icon = "pause"
	}
	mo.playBtn.Child().(*gtk.Label).SetText(icon)

	// Start/stop position ticker.
	mo.startTicker(mp.Playing)
}

func (mo *MediaOverlay) updatePosition(pos, dur float64) {
	mo.posLabel.SetText(surfaceutil.FormatTime(pos))
	mo.durLabel.SetText(surfaceutil.FormatTime(dur))
	// Block signals to prevent feedback loop.
	mo.scale.HandlerBlock(mo.changeHandle)
	mo.scale.SetValue(pos)
	mo.scale.HandlerUnblock(mo.changeHandle)
}

func (mo *MediaOverlay) startTicker(playing bool) {
	if mo.tickerCtx != nil {
		mo.tickerCtx()
		mo.tickerCtx = nil
	}
	if !playing || mo.player.PlayerName == "" {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	mo.tickerCtx = cancel
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pos, _ := mo.mprisSvc.GetPosition(mo.player.PlayerName)
				mo.bus.Publish(bus.TopicMediaTick, state.MediaTick{
					PlayerName: mo.player.PlayerName,
					Position:   pos,
					Duration:   mo.player.Duration,
					At:         time.Now(),
				})
			}
		}
	}()
}
