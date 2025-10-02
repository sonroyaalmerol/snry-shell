package bar

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const maxWorkspacePills = 10

type workspacesWidget struct {
	box   *gtk.Box
	pills []*gtk.Label
}

func newWorkspacesWidget(b *bus.Bus) gtk.Widgetter {
	w := &workspacesWidget{
		box: gtk.NewBox(gtk.OrientationHorizontal, 0),
	}
	w.box.AddCSSClass("workspaces")
	w.box.SetVAlign(gtk.AlignCenter)

	// Pre-create pills 1–10. CSS transitions handle all animation.
	for i := range maxWorkspacePills {
		label := gtk.NewLabel(fmt.Sprintf("%d", i+1))
		label.AddCSSClass("workspace-pill")
		w.box.Append(label)
		w.pills = append(w.pills, label)
	}

	b.Subscribe(bus.TopicWorkspaces, func(e bus.Event) {
		ws := e.Data.(state.Workspace)
		glib.IdleAdd(func() {
			w.update(ws)
		})
	})

	return w.box
}

func (w *workspacesWidget) update(ws state.Workspace) {
	idx := ws.ID - 1
	if idx < 0 || idx >= maxWorkspacePills {
		return
	}
	pill := w.pills[idx]

	// Active pill gets .active (grows wider via CSS min-width transition).
	// Remove active from all others first.
	for i, p := range w.pills {
		if i == idx {
			if ws.Active {
				p.AddCSSClass("active")
			} else {
				p.RemoveCSSClass("active")
			}
			if ws.Occupied {
				p.AddCSSClass("occupied")
			} else {
				p.RemoveCSSClass("occupied")
			}
		} else if ws.Active {
			// When a new workspace becomes active, deactivate all others.
			p.RemoveCSSClass("active")
		}
	}
	_ = pill
}
