package gtkutil

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

func ClearChildren(parent *gtk.Widget, remove func(gtk.Widgetter)) {
	var children []gtk.Widgetter
	for child := parent.FirstChild(); child != nil; {
		children = append(children, child)
		child = gtk.BaseWidget(child).NextSibling()
	}
	for _, child := range children {
		remove(child)
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
