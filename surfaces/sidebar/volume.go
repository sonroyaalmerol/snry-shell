package sidebar

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// newVolumeMixerWidget creates a per-app volume mixer widget.
func newVolumeMixerWidget(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.AddCSSClass("volume-mixer")

	header := gtk.NewLabel("Volume Mixer")
	header.AddCSSClass("notif-group-header")
	header.SetHAlign(gtk.AlignStart)
	box.Append(header)

	listBox := gtk.NewListBox()
	listBox.AddCSSClass("mixer-list")
	listBox.SetSelectionMode(gtk.SelectionNone)

	b.Subscribe(bus.TopicAudioMixer, func(e bus.Event) {
		ms, ok := e.Data.(state.AudioMixerState)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			child := listBox.FirstChild()
			for child != nil {
				next := child.(*gtk.Widget).NextSibling()
				listBox.Remove(child)
				child = next
			}

			for _, app := range ms.Apps {
				row := newMixerRow(refs, app)
				listBox.Append(row)
			}
		})
	})

	box.Append(listBox)
	return box
}

func newMixerRow(refs *servicerefs.ServiceRefs, app state.AudioApp) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 8)
	row.AddCSSClass("mixer-row")

	nameLabel := gtk.NewLabel(app.Name)
	nameLabel.AddCSSClass("mixer-app-name")
	nameLabel.SetHExpand(true)
	nameLabel.SetHAlign(gtk.AlignStart)
	nameLabel.SetEllipsize(3) // pango.EllipsizeEnd

	volLabel := gtk.NewLabel(fmt.Sprintf("%d%%", int(app.Volume*100)))
	volLabel.AddCSSClass("mixer-vol-label")

	scale := gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 1, 0.01)
	scale.AddCSSClass("mixer-slider")
	scale.SetDrawValue(false)
	scale.SetHExpand(true)

	changeHandle := scale.ConnectChangeValue(func(_ gtk.ScrollType, value float64) bool {
		if refs.AudioMixer != nil {
			go refs.AudioMixer.SetAppVolume(app.ID, value)
		}
		volLabel.SetText(fmt.Sprintf("%d%%", int(value*100)))
		return false
	})

	// Block the signal handler so initial SetValue doesn't trigger it.
	scale.HandlerBlock(changeHandle)
	scale.SetValue(app.Volume)
	scale.HandlerUnblock(changeHandle)

	muteBtn := gtk.NewButton()
	muteBtn.AddCSSClass("mixer-mute-btn")
	muteIcon := gtk.NewLabel("volume_up")
	if app.Muted {
		muteIcon.SetText("volume_off")
	}
	muteIcon.AddCSSClass("material-icon")
	muteBtn.SetChild(muteIcon)

	id := app.ID
	muteBtn.ConnectClicked(func() {
		if refs.AudioMixer != nil {
			go refs.AudioMixer.ToggleAppMute(id)
		}
	})

	row.Append(nameLabel)
	row.Append(volLabel)
	row.Append(scale)
	row.Append(muteBtn)

	return row
}
