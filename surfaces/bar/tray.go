package bar

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/services/sni"
)

// newTrayWidget creates the system tray area using icon buttons.
func newTrayWidget(b *bus.Bus) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 2)
	box.AddCSSClass("tray")

	b.Subscribe(bus.TopicTrayItems, func(e bus.Event) {
		items, ok := e.Data.([]*sni.TrayItem)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			gtkutil.ClearChildren(&box.Widget, box.Remove)

			for _, item := range items {
				btn := newTrayItemBtn(b, item)
				box.Append(btn)
			}
		})
	})

	return box
}

func newTrayItemBtn(b *bus.Bus, item *sni.TrayItem) gtk.Widgetter {
	btn := gtk.NewButton()
	btn.SetCursorFromName("pointer")
	btn.AddCSSClass("tray-item")
	btn.SetTooltipText(item.Title)
	if item.Title == "" {
		btn.SetTooltipText(item.ID)
	}

	icon := gtk.NewImage()
	icon.AddCSSClass("tray-icon")
	if item.IconName != "" {
		icon.SetFromIconName(item.IconName)
	} else {
		icon.SetFromIconName("application-x-executable")
	}
	icon.SetIconSize(1) // gtk.IconSizeNormal
	btn.SetChild(icon)

	key := item.BusName + string(item.Path)
	btn.ConnectClicked(func() {
		b.Publish(bus.TopicTrayActivate, key)
	})

	return btn
}
