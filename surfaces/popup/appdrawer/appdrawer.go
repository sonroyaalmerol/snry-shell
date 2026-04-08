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

// AppDrawer is a fullscreen overlay showing all installed apps in a grid.
type AppDrawer struct {
	win     *gtk.ApplicationWindow
	bus     *bus.Bus
	trigger gtk.Widgetter
	monitor *gdk.Monitor
	apps    []launcher.App
	current []launcher.App
	flowBox *gtk.FlowBox
	search  *gtk.SearchEntry
	scroll  *gtk.ScrolledWindow
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
	content.SetHAlign(gtk.AlignFill)
	content.SetVAlign(gtk.AlignFill)
	content.SetHExpand(true)
	content.SetMarginTop(layershell.BarHeight() + 16)
	layershell.OnBarHeightChanged(func(h int) {
		content.SetMarginTop(h + 16)
	})
	content.SetMarginStart(24)
	content.SetMarginEnd(24)

	// Header with title
	header := gtk.NewBox(gtk.OrientationHorizontal, 0)
	header.AddCSSClass("appdrawer-header")
	header.SetHAlign(gtk.AlignFill)
	header.SetMarginBottom(16)

	title := gtk.NewLabel("Apps")
	title.AddCSSClass("appdrawer-title")
	title.SetHAlign(gtk.AlignStart)
	title.SetHExpand(true)

	header.Append(title)

	// Search bar.
	d.search = gtk.NewSearchEntry()
	d.search.AddCSSClass("appdrawer-search-entry")
	d.search.SetHExpand(true)
	d.search.SetPlaceholderText("Search apps...")

	// Scrollable app grid.
	d.scroll = gtk.NewScrolledWindow()
	d.scroll.SetVExpand(true)
	d.scroll.SetHExpand(true)
	d.scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	d.scroll.AddCSSClass("appdrawer-scroll")

	gtkutil.SetupScrollHoverSuppression(d.scroll)

	d.flowBox = gtk.NewFlowBox()
	d.flowBox.AddCSSClass("appdrawer-grid")
	d.flowBox.SetHAlign(gtk.AlignFill)
	d.flowBox.SetHExpand(true)
	d.flowBox.SetSelectionMode(gtk.SelectionNone)
	d.flowBox.SetHomogeneous(true)
	d.flowBox.SetColumnSpacing(8)
	d.flowBox.SetRowSpacing(8)
	d.flowBox.SetMinChildrenPerLine(3)
	d.flowBox.SetMaxChildrenPerLine(20)

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

	d.search.ConnectActivate(func() {
		if len(d.current) == 0 {
			return
		}
		app := d.current[0]
		d.win.SetVisible(false)
		go func() {
			if err := launcher.Launch(app); err != nil {
				glib.IdleAdd(func() {
					gtkutil.ErrorDialog(d.win, "Launch failed", err.Error())
				})
			}
		}()
	})

	d.scroll.SetChild(d.flowBox)
	content.Append(header)
	content.Append(d.search)
	content.Append(d.scroll)

	// Wire up scrim click to dismiss.
	// Use PhaseTarget so this only fires for clicks directly on the scrim
	// background, not clicks that bubble up from the content area.
	clickGesture := gtk.NewGestureClick()
	clickGesture.SetButton(1)
	clickGesture.SetPropagationPhase(gtk.PhaseTarget)
	clickGesture.ConnectReleased(func(nPress int, x, y float64) {
		d.win.SetVisible(false)
	})
	scrim.AddController(clickGesture)

	scrim.Append(content)

	// Close on Escape - use PhaseCapture so it works even when search has focus.
	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.SetPropagationPhase(gtk.PhaseCapture)
	keyCtrl.ConnectKeyPressed(func(keyval, _ uint, _ gdk.ModifierType) bool {
		if keyval == 0xff1b { // GDK_KEY_Escape
			d.win.SetVisible(false)
			return true
		}
		return false
	})
	d.win.AddController(keyCtrl)

	d.win.SetChild(scrim)
}

func (d *AppDrawer) Toggle() {
	if d.win.Visible() {
		d.win.SetVisible(false)
	} else {
		if d.monitor != nil {
			layershell.SetMonitor(d.win, d.monitor)
		}
		// Scroll to top when opening
		if d.scroll != nil {
			d.scroll.SetVAdjustment(gtk.NewAdjustment(0, 0, 0, 0, 0, 0))
		}
		// Clear search when opening
		if d.search != nil {
			d.search.SetText("")
		}
		d.win.SetVisible(true)
		d.win.GrabFocus()
		d.search.GrabFocus()
	}
}

func (d *AppDrawer) clearGrid() {
	for {
		child := d.flowBox.ChildAtIndex(0)
		if child == nil {
			break
		}
		d.flowBox.Remove(child)
	}
}

func (d *AppDrawer) populateGrid(apps []launcher.App) {
	// Clear existing children
	d.clearGrid()
	d.current = apps

	// Show empty state if no apps
	if len(apps) == 0 {
		emptyBox := gtk.NewBox(gtk.OrientationVertical, 16)
		emptyBox.AddCSSClass("appdrawer-empty")
		emptyBox.SetHAlign(gtk.AlignCenter)
		emptyBox.SetVAlign(gtk.AlignCenter)

		emptyIcon := gtkutil.MaterialIcon("search_off")
		emptyIcon.AddCSSClass("appdrawer-empty-icon")
		emptyIcon.AddCSSClass("material-icon")

		emptyLabel := gtk.NewLabel("No apps found")
		emptyLabel.AddCSSClass("appdrawer-empty-text")

		emptyBox.Append(emptyIcon)
		emptyBox.Append(emptyLabel)
		d.flowBox.Append(emptyBox)
		return
	}

	for _, app := range apps {
		tile := newAppTile(app, d.win, func() {
			d.win.SetVisible(false)
		})
		d.flowBox.Append(tile)
	}
}

func newAppTile(app launcher.App, win *gtk.ApplicationWindow, onLaunch func()) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.AddCSSClass("appdrawer-tile")
	box.SetCursorFromName("pointer")

	// Icon container for better Material 3 styling
	iconBox := gtk.NewBox(gtk.OrientationVertical, 0)
	iconBox.SetHAlign(gtk.AlignCenter)
	iconBox.SetVAlign(gtk.AlignCenter)

	icon := gtk.NewImage()
	iconName := app.Icon
	if iconName == "" {
		iconName = "application-x-executable"
	}
	icon.SetFromIconName(iconName)
	icon.SetPixelSize(48)
	icon.AddCSSClass("appdrawer-tile-icon")

	iconBox.Append(icon)

	label := gtk.NewLabel(app.Name)
	label.SetJustify(gtk.JustifyCenter)
	label.SetWrap(true)
	label.SetMaxWidthChars(10)
	label.SetLines(2)
	label.SetEllipsize(2) // Pango.EllipsizeEnd
	label.AddCSSClass("appdrawer-tile-label")

	box.Append(iconBox)
	box.Append(label)

	gtkutil.ClaimedClick(&box.Widget, func() {
		onLaunch()
		go func() {
			if err := launcher.Launch(app); err != nil {
				glib.IdleAdd(func() {
					gtkutil.ErrorDialog(win, "Launch failed", err.Error())
				})
			}
		}()
	})

	return box
}

