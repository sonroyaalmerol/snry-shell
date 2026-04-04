package gtkutil

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

func ClearChildren(w *gtk.Widget) {
	var children []*gtk.Widget
	for child := w.FirstChild(); child != nil; {
		children = append(children, child)
		child = child.NextSibling()
	}
	for _, c := range children {
		w.Remove(c)
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
