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
}

func newControlPanel(cfg settings.Config) *controlPanel {
	return &controlPanel{
		cfg: cfg,
		providers: []ConfigProvider{
			newShellConfigProvider(&cfg),
		},
	}
}

func (cp *controlPanel) build() gtk.Widgetter {
	// Main horizontal box: sidebar + content
	root := gtk.NewBox(gtk.OrientationHorizontal, 0)
	root.AddCSSClass("control-panel-root")

	// Build sidebar navigation
	sidebar := cp.buildSidebar()
	root.Append(sidebar)

	// Build content area with stack
	content := cp.buildContent()
	root.Append(content)

	return root
}

func (cp *controlPanel) buildSidebar() gtk.Widgetter {
	sidebar := gtk.NewBox(gtk.OrientationVertical, 0)
	sidebar.AddCSSClass("control-panel-sidebar")
	sidebar.SetSizeRequest(280, -1)

	// Header
	header := gtk.NewBox(gtk.OrientationHorizontal, 12)
	header.AddCSSClass("control-panel-sidebar-header")
	header.SetMarginTop(24)
	header.SetMarginBottom(24)
	header.SetMarginStart(24)
	header.SetMarginEnd(24)

	title := gtk.NewLabel("Settings")
	title.AddCSSClass("control-panel-sidebar-title")
	title.SetHAlign(gtk.AlignStart)
	header.Append(title)

	sidebar.Append(header)

	// Navigation list
	list := gtk.NewListBox()
	list.AddCSSClass("control-panel-nav-list")
	list.SetSelectionMode(gtk.SelectionSingle)

	for i, provider := range cp.providers {
		row := cp.buildNavRow(provider, i == 0)
		list.Append(row)
	}

	// Connect selection change
	list.ConnectRowSelected(func(row *gtk.ListBoxRow) {
		if row != nil {
			idx := row.Index()
			cp.showProvider(idx)
		}
	})

	sidebar.Append(list)

	return sidebar
}

func (cp *controlPanel) buildNavRow(provider ConfigProvider, active bool) *gtk.ListBoxRow {
	row := gtk.NewListBoxRow()
	row.AddCSSClass("control-panel-nav-row")
	if active {
		row.AddCSSClass("active")
	}

	box := gtk.NewBox(gtk.OrientationHorizontal, 16)
	box.SetMarginStart(24)
	box.SetMarginEnd(24)
	box.SetMarginTop(16)
	box.SetMarginBottom(16)

	icon := gtk.NewLabel(provider.Icon())
	icon.AddCSSClass("material-icon")
	icon.AddCSSClass("control-panel-nav-icon")

	label := gtk.NewLabel(provider.Name())
	label.AddCSSClass("control-panel-nav-label")
	label.SetHAlign(gtk.AlignStart)
	label.SetHExpand(true)

	box.Append(icon)
	box.Append(label)
	row.SetChild(box)

	return row
}

func (cp *controlPanel) buildContent() gtk.Widgetter {
	content := gtk.NewBox(gtk.OrientationVertical, 0)
	content.AddCSSClass("control-panel-content")
	content.SetHExpand(true)

	// Stack for switching between providers
	cp.stack = gtk.NewStack()
	cp.stack.AddCSSClass("control-panel-stack")
	cp.stack.SetTransitionType(gtk.StackTransitionTypeSlideLeftRight)
	cp.stack.SetTransitionDuration(200)

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

// Stack reference for provider switching
func (cp *controlPanel) stackRef() *gtk.Stack {
	return cp.stack
}
