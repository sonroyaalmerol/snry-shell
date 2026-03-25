package sidebar

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// newWeatherWidget creates a small weather display widget.
func newWeatherWidget(b *bus.Bus) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 12)
	box.AddCSSClass("weather-widget")
	box.SetHAlign(gtk.AlignCenter)

	icon := gtk.NewLabel("cloud")
	icon.AddCSSClass("material-icon")
	icon.AddCSSClass("weather-icon")

	tempLabel := gtk.NewLabel("--°")
	tempLabel.AddCSSClass("weather-temp")

	condLabel := gtk.NewLabel("")
	condLabel.AddCSSClass("weather-condition")

	box.Append(icon)
	box.Append(tempLabel)
	box.Append(condLabel)

	b.Subscribe(bus.TopicWeather, func(e bus.Event) {
		ws, ok := e.Data.(state.WeatherState)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			icon.SetText(ws.Icon)
			tempLabel.SetText(fmt.Sprintf("%d°C", ws.TempC))
			condLabel.SetText(ws.Condition)
		})
	})

	return box
}
