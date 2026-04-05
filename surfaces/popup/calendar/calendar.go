package calendar

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
	"github.com/sonroyaalmerol/snry-shell/surfaces/widgets"
)

const (
	panelMargin = 12
	panelWidth  = 320
)

// Calendar is a popup showing a navigable month calendar.
type Calendar struct {
	win *gtk.ApplicationWindow
	bus *bus.Bus
}

// New creates and hides the calendar popup.
func New(app *gtk.Application, b *bus.Bus) *Calendar {
	win, _, root := surfaceutil.NewPopupPanel(app, b, surfaceutil.PopupPanelConfig{
		Name:      "snry-calendar",
		Namespace: "snry-calendar",
		CloseOn:   []string{"toggle-controls", "toggle-notif-center", "toggle-overview"},
		Align:     gtk.AlignEnd,
	})

	cal := &Calendar{win: win, bus: b}

	panel := gtk.NewBox(gtk.OrientationVertical, 0)
	panel.AddCSSClass("popup-panel")
	panel.SetMarginStart(panelMargin)
	panel.SetMarginEnd(panelMargin)
	panel.SetSizeRequest(panelWidth, -1)

	panel.Append(widgets.BuildCalendarGroup())

	root.Append(panel)

	surfaceutil.AddToggleOn(b, win, "toggle-calendar")

	return cal
}
