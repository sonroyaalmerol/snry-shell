// Package clipboard provides a clipboard history panel overlay.
package clipboard

import (
	"os/exec"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

// Panel is a clipboard history overlay anchored to the top-right.
type Panel struct {
	win    *gtk.ApplicationWindow
	list   *gtk.Box
	search *gtk.SearchEntry
	bus    *bus.Bus
}

func New(app *gtk.Application, b *bus.Bus) *Panel {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-clipboard",
		Layer:         layershell.LayerOverlay,
		Anchors:       map[layershell.Edge]bool{layershell.EdgeTop: true, layershell.EdgeRight: true},
		Margins:       map[layershell.Edge]int{layershell.EdgeTop: 48, layershell.EdgeRight: 8},
		KeyboardMode:  layershell.KeyboardModeOnDemand,
		ExclusiveZone: -1,
		Namespace:     "snry-clipboard",
	})

	p := &Panel{win: win, bus: b}

	root := gtk.NewBox(gtk.OrientationVertical, 0)
	root.AddCSSClass("clipboard-panel")
	root.SetSizeRequest(350, 400)

	// Search bar.
	searchBar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	searchBar.AddCSSClass("clipboard-search")

	p.search = gtk.NewSearchEntry()
	p.search.SetPlaceholderText("Search clipboard...")
	p.search.SetHExpand(true)
	p.search.ConnectSearchChanged(func() {
		p.refresh(p.search.Text())
	})

	clearBtn := gtkutil.MaterialButtonWithClass("delete_sweep", "clipboard-clear-btn")
	clearBtn.SetTooltipText("Clear all")
	clearBtn.ConnectClicked(func() {
		go func() { _ = exec.Command("cliphist", "wipe").Run() }()
		p.refresh("")
	})

	searchBar.Append(p.search)
	searchBar.Append(clearBtn)
	root.Append(searchBar)

	// Scrollable list.
	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)

	p.list = gtk.NewBox(gtk.OrientationVertical, 4)
	p.list.AddCSSClass("clipboard-list")
	scroll.SetChild(p.list)
	root.Append(scroll)

	win.SetChild(root)

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-clipboard" {
			glib.IdleAdd(func() {
				if win.Visible() {
					win.SetVisible(false)
				} else {
					p.refresh("")
					win.SetVisible(true)
					win.GrabFocus()
				}
			})
		}
	})

	surfaceutil.AddEscapeToClose(win)
	win.SetVisible(false)
	return p
}

func (p *Panel) refresh(filter string) {
	go func() {
		out, err := exec.Command("cliphist", "list").Output()
		if err != nil {
			return
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		glib.IdleAdd(func() {
			gtkutil.ClearChildren(&p.list.Widget, p.list.Remove)

			for _, line := range lines {
				if line == "" {
					continue
				}
				if filter != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(filter)) {
					continue
				}

				row := gtk.NewButton()
				row.SetCursorFromName("pointer")
				row.AddCSSClass("clipboard-row")

				lbl := gtk.NewLabel(line)
				lbl.AddCSSClass("clipboard-preview")
				lbl.SetEllipsize(3) // PANGO_ELLIPSIZE_END
				lbl.SetHAlign(gtk.AlignStart)
				lbl.SetXAlign(0)
				row.SetChild(lbl)

				text := line
				row.ConnectClicked(func() {
					go func() {
						_ = exec.Command("wl-copy", text).Run()
					}()
					p.win.SetVisible(false)
				})

				p.list.Append(row)
			}
		})
	}()
}
