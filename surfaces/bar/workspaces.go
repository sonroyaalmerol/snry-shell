package bar

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const maxWorkspacePills = 10

type workspacesWidget struct {
	box     *gtk.Box
	pills   []*gtk.Button
	querier *hyprland.Querier
}

func newWorkspacesWidget(b *bus.Bus, querier *hyprland.Querier) gtk.Widgetter {
	w := &workspacesWidget{
		box:     gtk.NewBox(gtk.OrientationHorizontal, 0),
		querier: querier,
	}
	w.box.AddCSSClass("workspaces")
	w.box.SetVAlign(gtk.AlignCenter)

	for i := range maxWorkspacePills {
		id := i + 1
		label := gtk.NewLabel(fmt.Sprintf("%d", id))
		label.AddCSSClass("workspace-pill-label")

		btn := gtk.NewButton()
		btn.SetCursorFromName("pointer")
		btn.AddCSSClass("workspace-pill")
		btn.SetChild(label)
		btn.SetTooltipText(fmt.Sprintf("Workspace %d", id))

		btn.ConnectClicked(func() {
			if w.querier != nil {
				go w.querier.SwitchWorkspace(id)
			}
		})

		w.box.Append(btn)
		w.pills = append(w.pills, btn)
	}

	// Right-click on the workspace area opens overview.
	rightClick := gtk.NewGestureClick()
	rightClick.SetButton(3)
	rightClick.ConnectPressed(func(_ int, _ float64, _ float64) {
		b.Publish(bus.TopicSystemControls, "toggle-overview")
	})
	w.box.AddController(rightClick)

	b.Subscribe(bus.TopicWorkspaces, func(e bus.Event) {
		ws := e.Data.(state.Workspace)
		glib.IdleAdd(func() { w.update(ws) })
	})

	return w.box
}

func (w *workspacesWidget) update(ws state.Workspace) {
	idx := ws.ID - 1
	if idx < 0 || idx >= maxWorkspacePills {
		return
	}
	for i, pill := range w.pills {
		if i == idx {
			if ws.Active {
				pill.AddCSSClass("active")
			} else {
				pill.RemoveCSSClass("active")
			}
			if ws.Occupied {
				pill.AddCSSClass("occupied")
			} else {
				pill.RemoveCSSClass("occupied")
			}
		} else if ws.Active {
			pill.RemoveCSSClass("active")
		}
	}
}
