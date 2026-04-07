package controlpanel

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
)

// controlPanel manages the control panel UI
type controlPanel struct {
	cfg       settings.Config
	providers []ConfigProvider
	stack     *gtk.Stack
	navList   *gtk.ListBox
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
	root := gtk.NewBox(gtk.OrientationHorizontal, 0)
	root.AddCSSClass("settings-panel")
	root.SetVExpand(true)
	root.SetHExpand(true)

	// Build sidebar navigation - use settings-nav style
	sidebar := cp.buildSidebar()
	root.Append(sidebar)

	// Build content area with stack - use settings-stack style
	content := cp.buildContent()
	root.Append(content)

	// Show first provider by default
	cp.showProvider(0)

	// Select first row
	if firstRow := cp.navList.RowAtIndex(0); firstRow != nil {
		cp.navList.SelectRow(firstRow)
	}

	return root
}

func (cp *controlPanel) buildSidebar() gtk.Widgetter {
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
