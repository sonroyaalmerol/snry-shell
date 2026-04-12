package overview

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/launcher"
)

func newSearchWidget(b *bus.Bus, onDismiss func()) gtk.Widgetter {
	entry := gtk.NewSearchEntry()
	entry.AddCSSClass("search-entry")
	entry.SetHExpand(true)
	entry.SetPlaceholderText("Search apps…")

	// Load all apps once; Filter on each keystroke.
	apps, _ := launcher.LoadAll()

	resultBox := gtk.NewBox(gtk.OrientationVertical, 4)
	resultBox.AddCSSClass("search-results")

	entry.ConnectSearchChanged(func() {
		query := entry.Text()
		// Clear previous results.
		for child := resultBox.FirstChild(); child != nil; child = resultBox.FirstChild() {
			resultBox.Remove(child)
		}
		if query == "" {
			return
		}
		filtered := launcher.Filter(apps, query)
		limit := min(len(filtered), 8)
		for _, app := range filtered[:limit] {
			row := newAppRow(app, onDismiss)
			resultBox.Append(row)
		}
	})

	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.Append(entry)
	box.Append(resultBox)
	return box
}

func newAppRow(app launcher.App, onActivate func()) gtk.Widgetter {
	btn := gtk.NewButton()
	btn.SetCursorFromName("pointer")
	btn.AddCSSClass("search-result-row")

	label := gtk.NewLabel(app.Name)
	label.SetHAlign(gtk.AlignStart)
	btn.SetChild(label)

	btn.ConnectClicked(func() {
		go launcher.Launch(app)
		onActivate()
	})
	return btn
}
