package bar

import (
	"fmt"
	"log"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
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
	labels  []*gtk.Label
	icons   []*gtk.Image
	querier *hyprland.Querier
	theme   *gtk.IconTheme
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

		image := gtk.NewImage()
		image.SetPixelSize(16)
		image.SetVisible(false)

		label := gtk.NewLabel(fmt.Sprintf("%d", id))
		label.AddCSSClass("workspace-pill-label")

		box := gtk.NewBox(gtk.OrientationHorizontal, 0)
		box.SetHAlign(gtk.AlignCenter)
		box.SetVAlign(gtk.AlignCenter)
		box.Append(image)
		box.Append(label)

		btn := gtk.NewButton()
		btn.SetCursorFromName("pointer")
		btn.AddCSSClass("workspace-pill")
		btn.SetChild(box)
		btn.SetTooltipText(fmt.Sprintf("Workspace %d", id))

		btn.ConnectClicked(func() {
			if w.querier != nil {
				go w.querier.SwitchWorkspace(id)
			}
		})

		w.box.Append(btn)
		w.pills = append(w.pills, btn)
		w.labels = append(w.labels, label)
		w.icons = append(w.icons, image)
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

	// Populate initial workspace icons from current clients.
	glib.IdleAdd(w.populateInitialIcons)

	return w.box
}

func (w *workspacesWidget) populateInitialIcons() {
	w.theme = gtk.IconThemeGetForDisplay(gdk.DisplayGetDefault())
	if w.querier == nil || w.theme == nil {
		return
	}
	clients, err := w.querier.Clients()
	if err != nil {
		log.Printf("[bar] workspace icons: %v", err)
		return
	}
	firstClass := make(map[int]string)
	for _, c := range clients {
		wsID := c.Workspace.ID
		if wsID < 1 || wsID > maxWorkspacePills {
			continue
		}
		if _, ok := firstClass[wsID]; !ok {
			firstClass[wsID] = c.Class
		}
	}
	for wsID, class := range firstClass {
		idx := wsID - 1
		w.setIcon(idx, class)
	}
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
			w.setIcon(i, ws.Icon)
		} else if ws.Active {
			pill.RemoveCSSClass("active")
		}
	}
}

func (w *workspacesWidget) setIcon(idx int, class string) {
	img := w.icons[idx]
	lbl := w.labels[idx]
	if class == "" {
		img.SetVisible(false)
		img.SetFromIconName("")
		lbl.SetVisible(true)
		return
	}
	if w.theme == nil {
		w.theme = gtk.IconThemeGetForDisplay(gdk.DisplayGetDefault())
	}
	if w.theme == nil {
		return
	}
	// GTK icon names are always lowercase; Hyprland classes can be mixed case.
	lower := strings.ToLower(class)
	if w.theme.HasIcon(lower) {
		img.SetFromIconName(lower)
		img.SetVisible(true)
		lbl.SetVisible(false)
	} else {
		img.SetVisible(false)
		img.SetFromIconName("")
		lbl.SetVisible(true)
	}
}
