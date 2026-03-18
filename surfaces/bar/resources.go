package bar

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

func newResourceIndicator(b *bus.Bus) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 2)
	box.AddCSSClass("status-indicator")
	box.SetTooltipText("System resources")

	icon := gtk.NewLabel("memory")
	icon.AddCSSClass("material-icon")

	label := gtk.NewLabel("")
	label.AddCSSClass("resource-indicator")

	box.Append(icon)
	box.Append(label)

	b.Subscribe(bus.TopicResources, func(e bus.Event) {
		rs := e.Data.(state.ResourceState)
		glib.IdleAdd(func() {
			text := fmt.Sprintf("%.0f%% %.0f%%", rs.CPU, rs.RAM)
			label.SetText(text)
			// Color coding via CSS class.
			if rs.CPU > 80 || rs.RAM > 80 {
				label.AddCSSClass("warning")
			} else {
				label.RemoveCSSClass("warning")
			}
		})
	})

	return box
}
