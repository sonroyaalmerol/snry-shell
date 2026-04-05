package gtkutil

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

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

// MaterialIcon returns a label styled as a Material Symbols icon.
func MaterialIcon(name string) *gtk.Label {
	l := gtk.NewLabel(name)
	l.AddCSSClass("material-icon")
	return l
}

// ConfirmDialog shows a modal confirmation dialog. Calls onConfirm if accepted.
func ConfirmDialog(parent *gtk.ApplicationWindow, title, message, action string, onConfirm func()) {
	dialog := gtk.NewMessageDialog(
		&parent.Window,
		gtk.DialogModal|gtk.DialogDestroyWithParent,
		gtk.MessageQuestion,
		gtk.ButtonsCancel,
	)
	dialog.SetTitle(title)
	dialog.SetMarkup(message)
	dialog.AddButton(action, int(gtk.ResponseAccept))

	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseAccept) {
			onConfirm()
		}
	})
	dialog.Show()
}

// SectionHeader creates a clickable header for a collapsible section.
func SectionHeader(title string, count int, revealer *gtk.Revealer, onScan func()) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, 8)
	box.AddCSSClass("conn-section-header")

	label := gtk.NewLabel(title)
	label.AddCSSClass("conn-section-title")
	label.SetHExpand(true)
	label.SetHAlign(gtk.AlignStart)

	countLabel := gtk.NewLabel(fmt.Sprintf("(%d)", count))
	countLabel.AddCSSClass("conn-section-count")

	arrow := MaterialIcon("expand_more")
	arrow.AddCSSClass("conn-section-arrow")

	box.Append(label)
	box.Append(countLabel)
	box.Append(arrow)

	box.SetVisible(count > 0)

	expanded := true
	click := gtk.NewGestureClick()
	click.SetButton(1)
	click.ConnectPressed(func(_ int, _ float64, _ float64) {
		click.SetState(gtk.EventSequenceClaimed)
	})
	click.ConnectReleased(func(_ int, _ float64, _ float64) {
		expanded = !expanded
		revealer.SetRevealChild(expanded)
		if expanded {
			arrow.SetText("expand_more")
		} else {
			arrow.SetText("expand_less")
		}
	})
	box.AddController(click)

	return box
}

// UpdateSectionHeader updates a section header's count and visibility.
func UpdateSectionHeader(header *gtk.Box, count int) {
	// Children: title, count, arrow — update the count label (2nd child).
	if child := header.FirstChild(); child != nil {
		if countChild := gtk.BaseWidget(child).NextSibling(); countChild != nil {
			if cl, ok := countChild.(*gtk.Label); ok {
				cl.SetText(fmt.Sprintf("(%d)", count))
			}
		}
	}
	header.SetVisible(count > 0)
}
