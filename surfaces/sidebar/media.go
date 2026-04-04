package sidebar

import (
	"context"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/services/mpris"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

type mediaControls struct {
	revealer     *gtk.Revealer
	box          *gtk.Box
	art          *gtk.Picture
	title        *gtk.Label
	artist       *gtk.Label
	scale        *gtk.Scale
	posLabel     *gtk.Label
	durLabel     *gtk.Label
	playBtn      *gtk.Button
	prevBtn      *gtk.Button
	nextBtn      *gtk.Button
	mprisSvc     *mpris.Service
	player       state.MediaPlayer
	bus          *bus.Bus
	tickerCtx    context.CancelFunc
	changeHandle glib.SignalHandle
}

func buildMediaGroup(b *bus.Bus, mprisSvc *mpris.Service) gtk.Widgetter {
	mc := &mediaControls{mprisSvc: mprisSvc, bus: b}

	mc.revealer = gtk.NewRevealer()
	mc.revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	mc.revealer.SetTransitionDuration(250)

	mc.box = gtk.NewBox(gtk.OrientationVertical, 0)
	mc.box.AddCSSClass("media-controls")

	// Top row: album art + text.
	top := gtk.NewBox(gtk.OrientationHorizontal, 8)
	mc.art = gtk.NewPicture()
	mc.art.AddCSSClass("media-album-art")

	info := gtk.NewBox(gtk.OrientationVertical, 2)
	mc.title = gtk.NewLabel("")
	mc.title.AddCSSClass("media-track-title")
	mc.title.SetHAlign(gtk.AlignStart)
	mc.artist = gtk.NewLabel("")
	mc.artist.AddCSSClass("media-artist")
	mc.artist.SetHAlign(gtk.AlignStart)

	info.Append(mc.title)
	info.Append(mc.artist)
	top.Append(mc.art)
	top.Append(info)

	// Progress row.
	progressRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	mc.posLabel = gtk.NewLabel("0:00")
	mc.posLabel.SetHAlign(gtk.AlignStart)
	mc.scale = gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 1, 0.001)
	mc.scale.AddCSSClass("media-progress")
	mc.scale.SetDrawValue(false)
	mc.scale.SetHExpand(true)
	mc.scale.SetSensitive(true)
	mc.changeHandle = mc.scale.ConnectChangeValue(func(_ gtk.ScrollType, value float64) bool {
		go mc.mprisSvc.SeekTo(mc.player.PlayerName, value*mc.player.Duration)
		return false
	})
	mc.durLabel = gtk.NewLabel("0:00")
	mc.durLabel.SetHAlign(gtk.AlignEnd)

	progressRow.Append(mc.posLabel)
	progressRow.Append(mc.scale)
	progressRow.Append(mc.durLabel)

	// Controls.
	controls := gtk.NewBox(gtk.OrientationHorizontal, 0)
	controls.SetHAlign(gtk.AlignCenter)
	controls.SetMarginTop(4)

	mc.prevBtn = gtkutil.MaterialButtonWithClass("skip_previous", "media-btn")
	mc.playBtn = mediaPlayBtn()
	mc.nextBtn = gtkutil.MaterialButtonWithClass("skip_next", "media-btn")

	mc.prevBtn.ConnectClicked(func() {
		go mc.mprisSvc.Previous(mc.player.PlayerName)
	})
	mc.playBtn.ConnectClicked(func() {
		go mc.mprisSvc.PlayPause(mc.player.PlayerName)
	})
	mc.nextBtn.ConnectClicked(func() {
		go mc.mprisSvc.Next(mc.player.PlayerName)
	})

	controls.Append(mc.prevBtn)
	controls.Append(mc.playBtn)
	controls.Append(mc.nextBtn)

	mc.box.Append(top)
	mc.box.Append(progressRow)
	mc.box.Append(controls)
	mc.revealer.SetChild(mc.box)

	// Subscribe to media state.
	b.Subscribe(bus.TopicMedia, func(e bus.Event) {
		mp := e.Data.(state.MediaPlayer)
		glib.IdleAdd(func() { mc.updatePlayer(mp) })
	})
	b.Subscribe(bus.TopicMediaTick, func(e bus.Event) {
		tick := e.Data.(state.MediaTick)
		glib.IdleAdd(func() { mc.updatePosition(tick.Position, tick.Duration) })
	})

	return mc.revealer
}

func (mc *mediaControls) updatePlayer(mp state.MediaPlayer) {
	mc.player = mp
	if mp.ArtPath != "" {
		mc.art.SetFilename(mp.ArtPath)
	}
	mc.title.SetText(mp.Title)
	mc.artist.SetText(mp.Artist)
	mc.durLabel.SetText(surfaceutil.FormatTime(mp.Duration))

	if mp.Duration > 0 {
		mc.scale.SetRange(0, mp.Duration)
	}
	mc.scale.HandlerBlock(mc.changeHandle)
	mc.scale.SetValue(mp.Position)
	mc.scale.HandlerUnblock(mc.changeHandle)

	icon := "play_arrow"
	if mp.Playing {
		icon = "pause"
	}
	mc.playBtn.Child().(*gtk.Label).SetText(icon)

	mc.startTicker(mp.Playing)
}

func (mc *mediaControls) updatePosition(pos, dur float64) {
	mc.posLabel.SetText(surfaceutil.FormatTime(pos))
	mc.durLabel.SetText(surfaceutil.FormatTime(dur))
	mc.scale.HandlerBlock(mc.changeHandle)
	mc.scale.SetValue(pos)
	mc.scale.HandlerUnblock(mc.changeHandle)
}

func (mc *mediaControls) startTicker(playing bool) {
	if mc.tickerCtx != nil {
		mc.tickerCtx()
		mc.tickerCtx = nil
	}
	if !playing || mc.player.PlayerName == "" {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	mc.tickerCtx = cancel
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pos, _ := mc.mprisSvc.GetPosition(mc.player.PlayerName)
				mc.bus.Publish(bus.TopicMediaTick, state.MediaTick{
					PlayerName: mc.player.PlayerName,
					Position:   pos,
					Duration:   mc.player.Duration,
					At:         time.Now(),
				})
			}
		}
	}()
}

func mediaPlayBtn() *gtk.Button {
	return gtkutil.MaterialButtonWithClass("play_arrow", "media-btn", "play-pause")
}
