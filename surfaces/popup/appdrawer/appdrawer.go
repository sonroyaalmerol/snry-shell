package appdrawer

import (
	"sort"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/launcher"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

const gridCols = 5

// AppDrawer is a fullscreen overlay showing all installed apps in a grid.
type AppDrawer struct {
	win      *gtk.ApplicationWindow
	bus      *bus.Bus
	trigger  gtk.Widgetter
	monitor  *gdk.Monitor
	apps     []launcher.App
	grid     *gtk.Grid
	search   *gtk.SearchEntry
	iconSize int
}

// New creates and hides the app drawer overlay.
func New(app *gtk.Application, b *bus.Bus, trigger gtk.Widgetter) *AppDrawer {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:         "snry-appdrawer",
		Layer:        layershell.LayerOverlay,
		Anchors:      layershell.FullscreenAnchors(),
		KeyboardMode: layershell.KeyboardModeOnDemand,
		Namespace:    "snry-appdrawer",
	})

	apps, _ := launcher.LoadAll()
	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})

	d := &AppDrawer{win: win, bus: b, trigger: trigger, apps: apps}
	d.build()

	win.SetVisible(false)

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

	// Close when other popups open.
	closeActions := []string{
		"toggle-notif-center", "toggle-wifi", "toggle-bluetooth",
		"toggle-calendar", "toggle-overview", "toggle-windowmgmt",
	}
	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		action, _ := e.Data.(string)
		for _, ca := range closeActions {
			if action == ca && win.Visible() {
				glib.IdleAdd(func() { win.SetVisible(false) })
				return
			}
		}
	})

	return d
}

func (d *AppDrawer) build() {
	// Scrim: dark translucent background, click to dismiss.
	scrim := gtk.NewBox(gtk.OrientationVertical, 0)
	scrim.AddCSSClass("appdrawer-scrim")

	// Main content area.
	content := gtk.NewBox(gtk.OrientationVertical, 0)
	content.AddCSSClass("appdrawer-content")
	content.SetHAlign(gtk.AlignCenter)
	content.SetVAlign(gtk.AlignFill)
	content.SetHExpand(true)
	content.SetMarginTop(layershell.BarExclusiveZone + 16)
	content.SetMarginStart(24)
	content.SetMarginEnd(24)

	// Determine icon size from monitor scale factor.
	d.iconSize = 48 // default for 1x
	if d.monitor != nil {
		scale := d.monitor.ScaleFactor()
		d.iconSize = int(48 * scale)
	}

	// Search bar.
	d.search = gtk.NewSearchEntry()
	d.search.AddCSSClass("appdrawer-search-entry")
	d.search.SetHExpand(true)
	d.search.SetPlaceholderText("Search apps...")

	// Scrollable app grid.
	scrolled := gtk.NewScrolledWindow()
	scrolled.SetVExpand(true)
	scrolled.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scrolled.AddCSSClass("appdrawer-scroll")

	d.grid = gtk.NewGrid()
	d.grid.AddCSSClass("appdrawer-grid")
	d.grid.SetHAlign(gtk.AlignCenter)
	d.grid.SetColumnSpacing(8)
	d.grid.SetRowSpacing(8)

	d.populateGrid(d.apps)

	d.search.ConnectSearchChanged(func() {
		query := d.search.Text()
		d.clearGrid()
		if query == "" {
			d.populateGrid(d.apps)
			return
		}
		d.populateGrid(launcher.Filter(d.apps, query))
	})

	scrolled.SetChild(d.grid)
	content.Append(d.search)
	content.Append(scrolled)

	// Wire up scrim click to dismiss.
	clickGesture := gtk.NewGestureClick()
	clickGesture.SetButton(1)
	clickGesture.SetPropagationLimit(gtk.LimitNone)
	clickGesture.ConnectReleased(func(nPress int, x, y float64) {
		d.win.SetVisible(false)
	})
	scrim.AddController(clickGesture)

	scrim.Append(content)
	surfaceutil.AddEscapeToClose(d.win)
	d.win.SetChild(scrim)
}

func (d *AppDrawer) Toggle() {
	if d.win.Visible() {
		d.win.SetVisible(false)
	} else {
		if d.monitor != nil {
			layershell.SetMonitor(d.win, d.monitor)
		}
		d.win.SetVisible(true)
		d.win.GrabFocus()
		d.search.GrabFocus()
	}
}

func (d *AppDrawer) clearGrid() {
	for child := d.grid.FirstChild(); child != nil; child = d.grid.FirstChild() {
		d.grid.Remove(child)
	}
}

func (d *AppDrawer) populateGrid(apps []launcher.App) {
	for i, app := range apps {
		col := i % gridCols
		row := i / gridCols
		tile := newAppTile(app, d.iconSize, func() {
			d.win.SetVisible(false)
		})
		tile.SetHExpand(true)
		tile.SetVExpand(true)
		tile.SetHAlign(gtk.AlignCenter)
		d.grid.Attach(tile, col, row, 1, 1)
	}
}

func newAppTile(app launcher.App, iconSize int, onLaunch func()) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 6)
	box.AddCSSClass("appdrawer-tile")
	box.SetCursorFromName("pointer")

	icon := gtk.NewImage()
	iconName := app.Icon
	if iconName == "" {
		iconName = "application-x-executable"
	}
	icon.SetFromIconName(iconName)
	icon.SetIconSize(6) // gtk.IconSizeLarge (48px base, auto-scales with DPI)
	icon.AddCSSClass("appdrawer-tile-icon")

	label := gtk.NewLabel(app.Name)
	label.AddCSSClass("appdrawer-tile-label")

	box.Append(icon)
	box.Append(label)

	gtkutil.ClaimedClick(&box.Widget, func() {
		go launcher.Launch(app)
		onLaunch()
	})

	return box
}
