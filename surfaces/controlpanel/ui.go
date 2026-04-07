package controlpanel

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
)

// controlPanel manages the control panel UI
type controlPanel struct {
	cfg            settings.Config
	providers      []ConfigProvider
	stack          *gtk.Stack
	navList        *gtk.ListBox
	sidebar        *gtk.Box
	root           *gtk.Box
	menuBtn        *gtk.Button
	sidebarVisible bool
}

func newControlPanel(cfg settings.Config) *controlPanel {
	cp := &controlPanel{
		cfg: cfg,
		providers: []ConfigProvider{
			newShellConfigProvider(&cfg),
		},
	}

	// Try to add network provider if we can connect to D-Bus
	if nmProvider := newNMProviderWithConnection(); nmProvider != nil {
		cp.providers = append(cp.providers, nmProvider)
	}

	return cp
}

func (cp *controlPanel) build() gtk.Widgetter {
	// Main horizontal box: sidebar + content - use settings-panel style
	cp.root = gtk.NewBox(gtk.OrientationHorizontal, 0)
	cp.root.AddCSSClass("settings-panel")
	cp.root.SetVExpand(true)
	cp.root.SetHExpand(true)

	// Build sidebar navigation - use settings-nav style
	cp.sidebar = cp.buildSidebar()
	cp.root.Append(cp.sidebar)

	// Build content area with stack - use settings-stack style
	content := cp.buildContent()
	cp.root.Append(content)

	// Show first provider by default
	cp.showProvider(0)

	// Select first row
	if firstRow := cp.navList.RowAtIndex(0); firstRow != nil {
		cp.navList.SelectRow(firstRow)
	}

	// Set up responsive behavior
	cp.setupResponsive()

	return cp.root
}

func (cp *controlPanel) setupResponsive() {
	// Watch for widget size changes
	cp.root.ConnectMap(func() {
		cp.updateSidebarVisibility()
		cp.watchWindowSize()
	})

	// Also update on realize
	cp.root.ConnectRealize(func() {
		glib.IdleAdd(func() {
			cp.updateSidebarVisibility()
			cp.watchWindowSize()
		})
	})
}

func (cp *controlPanel) watchWindowSize() {
	// Use a timeout to periodically check window size
	// This is more reliable than resize events in GTK4
	var lastWidth int
	glib.TimeoutAdd(250, func() bool {
		window := cp.root.Root()
		if window == nil {
			return true // Keep checking
		}
		w := window.AllocatedWidth()
		if w != lastWidth {
			lastWidth = w
			cp.updateSidebarVisibility()
		}
		return true // Continue timer
	})
}

func (cp *controlPanel) updateSidebarVisibility() {
	// Get window width
	window := cp.root.Root()
	if window == nil {
		return
	}

	w := window.AllocatedWidth()

	// Breakpoint: auto-collapse sidebar when window is narrower than 700px
	// Auto-expand when wider than 700px
	if w < 700 {
		cp.sidebar.SetVisible(false)
		cp.sidebarVisible = false
	} else if w >= 700 {
		cp.sidebar.SetVisible(true)
		cp.sidebarVisible = true
	}
}

func (cp *controlPanel) toggleSidebar() {
	cp.sidebarVisible = !cp.sidebarVisible
	cp.sidebar.SetVisible(cp.sidebarVisible)
}

func (cp *controlPanel) buildSidebar() *gtk.Box {
	// Use the existing settings-nav style from shell
	sidebar := gtk.NewBox(gtk.OrientationVertical, 0)
	sidebar.AddCSSClass("settings-nav")
	sidebar.SetSizeRequest(280, -1)

	// Header with back button style from shell
	header := gtk.NewBox(gtk.OrientationHorizontal, 12)
	header.SetMarginTop(24)
	header.SetMarginBottom(24)
	header.SetMarginStart(24)
	header.SetMarginEnd(24)

	title := gtk.NewLabel("Settings")
	title.AddCSSClass("notif-group-header")
	title.SetHAlign(gtk.AlignStart)
	header.Append(title)

	sidebar.Append(header)

	// Navigation list - reuse settings style
	cp.navList = gtk.NewListBox()
	cp.navList.SetSelectionMode(gtk.SelectionSingle)

	for i, provider := range cp.providers {
		row := cp.buildNavRow(provider, i == 0)
		cp.navList.Append(row)
	}

	// Connect selection change
	cp.navList.ConnectRowSelected(func(row *gtk.ListBoxRow) {
		if row != nil {
			idx := row.Index()
			cp.showProvider(idx)
			// Auto-hide sidebar on mobile when selecting an item
			window := cp.root.Root()
			if window != nil && window.AllocatedWidth() < 700 {
				cp.sidebarVisible = false
				cp.sidebar.SetVisible(false)
			}
		}
	})

	sidebar.Append(cp.navList)

	return sidebar
}

func (cp *controlPanel) buildNavRow(provider ConfigProvider, active bool) *gtk.ListBoxRow {
	row := gtk.NewListBoxRow()
	row.AddCSSClass("settings-nav-item")
	if active {
		// The list will handle selection visually
	}

	box := gtk.NewBox(gtk.OrientationHorizontal, 16)
	box.SetMarginStart(16)
	box.SetMarginEnd(16)
	box.SetMarginTop(12)
	box.SetMarginBottom(12)

	icon := gtk.NewLabel(provider.Icon())
	icon.AddCSSClass("material-icon")
	icon.AddCSSClass("quick-slider-icon")

	label := gtk.NewLabel(provider.Name())
	label.AddCSSClass("settings-nav-label")
	label.SetHAlign(gtk.AlignStart)
	label.SetHExpand(true)

	box.Append(icon)
	box.Append(label)
	row.SetChild(box)

	return row
}

func (cp *controlPanel) buildContent() gtk.Widgetter {
	// Use settings-stack style from shell
	content := gtk.NewBox(gtk.OrientationVertical, 0)
	content.AddCSSClass("settings-stack")
	content.SetHExpand(true)
	content.SetVExpand(true)

	// Header with menu button for mobile
	header := gtk.NewBox(gtk.OrientationHorizontal, 12)
	header.SetMarginTop(16)
	header.SetMarginBottom(16)
	header.SetMarginStart(16)
	header.SetMarginEnd(16)

	// Menu button to toggle sidebar on mobile
	cp.menuBtn = gtkutil.M3IconButton("menu", "settings-btn-small")
	cp.menuBtn.SetTooltipText("Show menu")
	cp.menuBtn.ConnectClicked(func() {
		cp.toggleSidebar()
	})
	header.Append(cp.menuBtn)

	// Title
	title := gtk.NewLabel("Control Panel")
	title.AddCSSClass("settings-title")
	title.SetHAlign(gtk.AlignStart)
	header.Append(title)

	content.Append(header)

	// Stack for switching between providers
	cp.stack = gtk.NewStack()
	cp.stack.SetTransitionType(gtk.StackTransitionTypeSlideLeftRight)
	cp.stack.SetTransitionDuration(200)
	cp.stack.SetVExpand(true)
	cp.stack.SetHExpand(true)

	for i, provider := range cp.providers {
		widget := provider.BuildWidget()
		cp.stack.AddNamed(widget, providerName(i))
	}

	content.Append(cp.stack)

	return content
}

func (cp *controlPanel) showProvider(index int) {
	name := providerName(index)
	cp.stack.SetVisibleChildName(name)
}

func providerName(index int) string {
	return fmt.Sprintf("provider-%d", index)
}
