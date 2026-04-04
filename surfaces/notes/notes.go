// Package notes provides a floating notes overlay with persistent storage.
package notes

import (
	"os"
	"path/filepath"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

const notesPath = ".local/share/snry-shell/notes.txt"

// Overlay is a centered floating notes overlay.
type Overlay struct {
	win  *gtk.ApplicationWindow
	buf  *gtk.TextBuffer
	bus  *bus.Bus
	path string
}

func New(app *gtk.Application, b *bus.Bus) *Overlay {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-notes",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeExclusive,
		ExclusiveZone: -1,
		Namespace:     "snry-notes",
	})

	root := gtk.NewBox(gtk.OrientationVertical, 0)
	root.AddCSSClass("notes-overlay")
	root.SetSizeRequest(450, 400)

	// Toolbar.
	toolbar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	toolbar.AddCSSClass("notes-toolbar")

	title := gtk.NewLabel("Notes")
	title.AddCSSClass("notes-title")
	title.SetHExpand(true)
	title.SetHAlign(gtk.AlignStart)

	clearBtn := gtkutil.MaterialButtonWithClass("delete", "notes-action-btn")
	clearBtn.SetTooltipText("Clear")

	saveBtn := gtkutil.MaterialButtonWithClass("save", "notes-action-btn")
	saveBtn.SetTooltipText("Save")

	toolbar.Append(title)
	toolbar.Append(clearBtn)
	toolbar.Append(saveBtn)
	root.Append(toolbar)

	// Text view.
	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)

	p := &Overlay{win: win, bus: b}

	p.buf = gtk.NewTextBuffer(nil)
	tv := gtk.NewTextViewWithBuffer(p.buf)
	tv.AddCSSClass("notes-text")
	tv.SetWrapMode(2) // GTK_WRAP_WORD_CHAR
	scroll.SetChild(tv)

	root.Append(scroll)
	win.SetChild(root)

	home, _ := os.UserHomeDir()
	p.path = filepath.Join(home, notesPath)

	saveBtn.ConnectClicked(func() {
		p.save()
	})

	clearBtn.ConnectClicked(func() {
		p.buf.SetText("")
		p.save()
	})

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-notes" {
			glib.IdleAdd(func() {
				if win.Visible() {
					p.save()
					win.SetVisible(false)
				} else {
					p.load()
					win.SetVisible(true)
					tv.GrabFocus()
				}
			})
		}
	})

	surfaceutil.AddEscapeToCloseWithCallback(win, p.save)
	win.SetVisible(false)
	return p
}

func (n *Overlay) load() {
	data, err := os.ReadFile(n.path)
	if err == nil {
		start := n.buf.StartIter()
		end := n.buf.EndIter()
		n.buf.Delete(start, end)
		n.buf.Insert(start, string(data))
	}
}

func (n *Overlay) save() {
	start := n.buf.StartIter()
	end := n.buf.EndIter()
	text := n.buf.Text(start, end, false)

	dir := filepath.Dir(n.path)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(n.path, []byte(text), 0o644)
}
