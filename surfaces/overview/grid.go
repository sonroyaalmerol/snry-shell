package overview

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
)

// newGridWidget creates the window-preview grid populated with Hyprland clients.
func newGridWidget(b *bus.Bus, querier *hyprland.Querier) *gridWidget {
	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)

	flow := gtk.NewFlowBox()
	flow.AddCSSClass("window-grid")
	flow.SetColumnSpacing(12)
	flow.SetRowSpacing(12)
	flow.SetSelectionMode(gtk.SelectionNone)
	flow.SetMaxChildrenPerLine(6)

	scroll.SetChild(flow)

	widget := &gridWidget{scroll: scroll, flow: flow, querier: querier}
	return widget
}

type gridWidget struct {
	scroll  *gtk.ScrolledWindow
	flow    *gtk.FlowBox
	querier *hyprland.Querier
}

func (g *gridWidget) refresh() {
	if g.querier == nil {
		return
	}

	clients, err := g.querier.Clients()
	if err != nil {
		return
	}

	// Remove old children.
	child := g.flow.FirstChild()
	for child != nil {
		next := child.(*gtk.Widget).NextSibling()
		g.flow.Remove(child)
		child = next
	}

	// Group clients by workspace.
	wsClients := make(map[int][]hyprland.HyprClient)
	for _, c := range clients {
		wsID := c.Workspace.ID
		wsClients[wsID] = append(wsClients[wsID], c)
	}

	for wsID, cls := range wsClients {
		// Workspace label.
		wsLabel := gtk.NewLabel("")
		wsLabel.AddCSSClass("window-grid-ws-label")
		_ = wsID
		g.flow.Append(wsLabel)

		for _, c := range cls {
			card := newWindowCard(g.querier, c)
			g.flow.Append(card)
		}
	}
}

func newWindowCard(querier *hyprland.Querier, client hyprland.HyprClient) gtk.Widgetter {
	btn := gtk.NewButton()
	btn.AddCSSClass("window-preview-card")

	card := gtk.NewBox(gtk.OrientationVertical, 4)
	card.SetHAlign(gtk.AlignFill)

	// Placeholder area for thumbnail.
	thumb := gtk.NewBox(gtk.OrientationVertical, 0)
	thumb.AddCSSClass("window-preview-thumb")
	thumb.SetSizeRequest(200, 120)

	// Title.
	title := gtk.NewLabel(client.Title)
	title.AddCSSClass("window-preview-title")
	title.SetEllipsize(3) // pango.EllipsizeEnd
	title.SetHAlign(gtk.AlignStart)

	// Class/app name.
	classLabel := gtk.NewLabel(client.Class)
	classLabel.AddCSSClass("window-preview-class")
	classLabel.SetHAlign(gtk.AlignStart)

	card.Append(thumb)
	card.Append(title)
	card.Append(classLabel)
	btn.SetChild(card)

	addr := client.Address
	btn.ConnectClicked(func() {
		if querier != nil {
			_ = querier.FocusWindow(addr)
		}
	})

	return btn
}
