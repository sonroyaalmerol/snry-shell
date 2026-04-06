package appdrawer

import (
	"sort"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/launcher"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

const (
	panelMargin = 12
	panelWidth  = 400
)

// AppDrawer is a popup panel showing an Android-style app grid with search.
type AppDrawer struct {
	win      *gtk.ApplicationWindow
	bus      *bus.Bus
	trigger  gtk.Widgetter
	monitor  *gdk.Monitor
	root     *gtk.Box
	flowBox  *gtk.FlowBox
	apps     []launcher.App
}

// New creates and hides the app drawer popup anchored to the given trigger widget.
func New(app *gtk.Application, b *bus.Bus, trigger gtk.Widgetter) *AppDrawer {
	win, _, root := surfaceutil.NewPopupPanel(app, b, surfaceutil.PopupPanelConfig{
		Name:      "snry-appdrawer",
		Namespace: "snry-appdrawer",
		CloseOn:   []string{"toggle-notif-center", "toggle-wifi", "toggle-bluetooth", "toggle-calendar", "toggle-overview", "toggle-windowmgmt"},
		Align:     gtk.AlignStart,
	})

	apps, _ := launcher.LoadAll()
	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})

	d := &AppDrawer{win: win, bus: b, trigger: trigger, root: root, apps: apps}

	panel := gtk.NewBox(gtk.OrientationVertical, 0)
	panel.AddCSSClass("popup-panel")
	panel.AddCSSClass("app-drawer-panel")
	panel.SetMarginStart(panelMargin)
	panel.SetMarginEnd(panelMargin)
	panel.SetSizeRequest(panelWidth, -1)

	// Search bar.
	search := gtk.NewSearchEntry()
	search.AddCSSClass("app-search-entry")
	search.SetPlaceholderText("Search apps...")

	// Scrollable app grid.
	scrolled := gtk.NewScrolledWindow()
	scrolled.SetHExpand(true)
	scrolled.SetVExpand(true)
	scrolled.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scrolled.AddCSSClass("popup-scroll")

	d.flowBox = gtk.NewFlowBox()
	d.flowBox.AddCSSClass("app-grid")
	d.flowBox.SetHomogeneous(true)
	d.flowBox.SetColumnSpacing(4)
	d.flowBox.SetRowSpacing(4)
	d.flowBox.SetMaxChildrenPerLine(4)
	d.flowBox.SetSelectionMode(gtk.SelectionNone)

	populateGrid(d.flowBox, apps, func() {
		d.win.SetVisible(false)
	})

	search.ConnectSearchChanged(func() {
		query := search.Text()
		// Clear grid.
		for child := d.flowBox.FirstChild(); child != nil; child = d.flowBox.FirstChild() {
			d.flowBox.Remove(child)
		}
		if query == "" {
			populateGrid(d.flowBox, apps, func() { d.win.SetVisible(false) })
			return
		}
		filtered := launcher.Filter(apps, query)
		populateGrid(d.flowBox, filtered, func() { d.win.SetVisible(false) })
	})

	scrolled.SetChild(d.flowBox)
	panel.Append(search)
	panel.Append(scrolled)

	root.Append(panel)

	b.Subscribe(bus.TopicPopupTrigger, func(e bus.Event) {
		pt, ok := e.Data.(surfaceutil.PopupTrigger)
		if !ok {
			return
		}
		if pt.Action == "toggle-appdrawer" {
			d.trigger = pt.Trigger
			d.monitor = pt.Monitor
		}
	})

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-appdrawer" {
			glib.IdleAdd(func() { d.Toggle() })
		}
	})

	return d
}

func (d *AppDrawer) Toggle() {
	if d.win.Visible() {
		d.win.SetVisible(false)
	} else {
		if d.monitor != nil {
			layershell.SetMonitor(d.win, d.monitor)
		}
		surfaceutil.PositionUnderTrigger(d.root, d.trigger, panelWidth, panelMargin, d.monitor)
		d.win.SetVisible(true)
	}
}

func populateGrid(flowBox *gtk.FlowBox, apps []launcher.App, onLaunch func()) {
	for _, app := range apps {
		flowBox.Append(newAppIcon(app, onLaunch))
	}
}

func newAppIcon(app launcher.App, onLaunch func()) gtk.Widgetter {
	btn := gtk.NewButton()
	btn.SetCursorFromName("pointer")
	btn.AddCSSClass("app-icon-btn")

	icon := gtk.NewImage()
	iconName := app.Icon
	if iconName == "" {
		iconName = "application-x-executable"
	}
	icon.SetFromIconName(iconName)
	icon.SetIconSize(6) // gtk.IconSizeLarge
	icon.AddCSSClass("app-icon-image")

	label := gtk.NewLabel(app.Name)
	label.AddCSSClass("app-icon-label")
	label.SetEllipsize(3) // pango.EllipsizeEnd
	label.SetMaxWidthChars(10)

	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.SetHAlign(gtk.AlignCenter)
	box.Append(icon)
	box.Append(label)
	btn.SetChild(box)

	btn.ConnectClicked(func() {
		go launcher.Launch(app)
		onLaunch()
	})

	return btn
}
