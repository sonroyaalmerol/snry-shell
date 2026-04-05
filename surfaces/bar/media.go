package bar

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// newMediaWidget returns a GtkRevealer containing a clickable media pill.
// It is hidden unless a player is actively playing.
func newMediaWidget(b *bus.Bus) gtk.Widgetter {
	revealer := gtk.NewRevealer()
	revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideLeft)
	revealer.SetTransitionDuration(200)
	revealer.SetRevealChild(false)

	btn := gtk.NewButton()
	btn.SetCursorFromName("pointer")
	btn.AddCSSClass("media-pill")
	btn.SetHAlign(gtk.AlignCenter)

	label := gtk.NewLabel("")
	label.SetEllipsize(3) // pango.EllipsizeEnd
	label.SetMaxWidthChars(30)
	btn.SetChild(label)

	b.Subscribe(bus.TopicMedia, func(e bus.Event) {
		mp := e.Data.(state.MediaPlayer)
		glib.IdleAdd(func() {
			if !mp.Playing || mp.Title == "" {
				revealer.SetRevealChild(false)
				return
			}
			text := mp.Title
			if mp.Artist != "" {
				text = fmt.Sprintf("%s — %s", mp.Artist, mp.Title)
			}
			label.SetText(text)
			revealer.SetRevealChild(true)
		})
	})

	btn.ConnectClicked(func() {
		b.Publish(bus.TopicSystemControls, "toggle-media-overlay")
	})

	revealer.SetChild(btn)
	return revealer
}
