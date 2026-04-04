package gtkutil

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

func ClearChildren(w *gtk.Widget) {
	for child := w.FirstChild(); child != nil; {
		next := child.(*gtk.Widget).NextSibling()
		child.(*gtk.Widget).SetParent(nil)
		child = next
	}
}

func MaterialButton(icon string) *gtk.Button {
	btn := gtk.NewButton()
	label := gtk.NewLabel(icon)
	label.AddCSSClass("material-icon")
	btn.SetChild(label)
	return btn
}

func MaterialButtonWithClass(icon string, classes ...string) *gtk.Button {
	btn := MaterialButton(icon)
	for _, c := range classes {
		btn.AddCSSClass(c)
	}
	return btn
}
