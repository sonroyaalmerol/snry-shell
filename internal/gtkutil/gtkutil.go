package gtkutil

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
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
func MaterialIcon(name string, classes ...string) *gtk.Label {
	l := gtk.NewLabel(name)
	l.AddCSSClass("material-icon")
	for _, c := range classes {
		l.AddCSSClass(c)
	}
	return l
}

// ConfirmDialog shows an M3-styled confirmation dialog as a layer-shell overlay.
// Calls onConfirm if the action button is clicked, or dismisses on outside click / Escape.
func ConfirmDialog(parent *gtk.ApplicationWindow, title, message, action string, onConfirm func()) {
	win := layershell.NewWindow(parent.Application(), layershell.WindowConfig{
		Name:          "snry-m3-dialog",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeOnDemand,
		ExclusiveZone: -1,
		Namespace:     "snry-m3-dialog",
	})

	close := func() { win.SetVisible(false) }

	// Scrim background — clicking it dismisses the dialog.
	scrim := gtk.NewBox(gtk.OrientationVertical, 0)
	scrim.AddCSSClass("m3-dialog-scrim")
	scrim.SetHExpand(true)
	scrim.SetVExpand(true)
	clickGesture := gtk.NewGestureClick()
	clickGesture.SetButton(1)
	clickGesture.SetPropagationLimit(gtk.LimitNone)
	clickGesture.ConnectReleased(func(_ int, _ float64, _ float64) {
		close()
	})
	scrim.AddController(clickGesture)

	// Centered dialog card.
	centerBox := gtk.NewBox(gtk.OrientationVertical, 0)
	centerBox.SetHAlign(gtk.AlignCenter)
	centerBox.SetVAlign(gtk.AlignCenter)
	centerBox.SetVExpand(true)

	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("m3-dialog")

	titleLabel := gtk.NewLabel(title)
	titleLabel.AddCSSClass("m3-dialog-title")
	titleLabel.SetWrap(true)
	titleLabel.SetXAlign(0)
	card.Append(titleLabel)

	if message != "" {
		msgLabel := gtk.NewLabel(message)
		msgLabel.AddCSSClass("m3-dialog-content")
		msgLabel.SetWrap(true)
		msgLabel.SetXAlign(0)
		card.Append(msgLabel)
	}

	btnBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
	btnBox.AddCSSClass("m3-dialog-actions")

	cancelBtn := gtk.NewButtonWithLabel("Cancel")
	cancelBtn.AddCSSClass("m3-dialog-btn")

	actionBtn := gtk.NewButtonWithLabel(action)
	actionBtn.AddCSSClass("m3-dialog-btn")

	btnBox.Append(cancelBtn)
	btnBox.Append(actionBtn)
	card.Append(btnBox)

	centerBox.Append(card)
	scrim.Append(centerBox)
	win.SetChild(scrim)

	cancelBtn.ConnectClicked(close)
	actionBtn.ConnectClicked(func() {
		close()
		onConfirm()
	})

	surfaceutil.AddEscapeToClose(win)
	win.SetVisible(true)
}

// ActionDialogAction describes a single action button in an ActionDialog.
type ActionDialogAction struct {
	Label    string
	CSSClass string
	OnClick  func()
}

// ActionDialog shows an M3-styled dialog with multiple action buttons as a
// layer-shell overlay.
func ActionDialog(parent *gtk.ApplicationWindow, title, message string, actions []ActionDialogAction) {
	win := layershell.NewWindow(parent.Application(), layershell.WindowConfig{
		Name:          "snry-m3-action-dialog",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeOnDemand,
		ExclusiveZone: -1,
		Namespace:     "snry-m3-dialog",
	})

	close := func() { win.SetVisible(false) }

	// Scrim background.
	scrim := gtk.NewBox(gtk.OrientationVertical, 0)
	scrim.AddCSSClass("m3-dialog-scrim")
	scrim.SetHExpand(true)
	scrim.SetVExpand(true)
	clickGesture := gtk.NewGestureClick()
	clickGesture.SetButton(1)
	clickGesture.SetPropagationLimit(gtk.LimitNone)
	clickGesture.ConnectReleased(func(_ int, _ float64, _ float64) { close() })
	scrim.AddController(clickGesture)

	// Centered dialog card.
	centerBox := gtk.NewBox(gtk.OrientationVertical, 0)
	centerBox.SetHAlign(gtk.AlignCenter)
	centerBox.SetVAlign(gtk.AlignCenter)
	centerBox.SetVExpand(true)

	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("m3-dialog")

	titleLabel := gtk.NewLabel(title)
	titleLabel.AddCSSClass("m3-dialog-title")
	titleLabel.SetWrap(true)
	titleLabel.SetXAlign(0)
	card.Append(titleLabel)

	if message != "" {
		msgLabel := gtk.NewLabel(message)
		msgLabel.AddCSSClass("m3-dialog-content")
		msgLabel.SetWrap(true)
		msgLabel.SetXAlign(0)
		card.Append(msgLabel)
	}

	btnBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
	btnBox.AddCSSClass("m3-dialog-actions")

	for _, action := range actions {
		actionBtn := gtk.NewButtonWithLabel(action.Label)
		actionBtn.AddCSSClass("m3-dialog-btn")
		if action.CSSClass != "" {
			actionBtn.AddCSSClass(action.CSSClass)
		}
		act := action
		actionBtn.ConnectClicked(func() {
			close()
			act.OnClick()
		})
		btnBox.Append(actionBtn)
	}

	cancelBtn := gtk.NewButtonWithLabel("Cancel")
	cancelBtn.AddCSSClass("m3-dialog-btn")
	btnBox.Append(cancelBtn)
	cancelBtn.ConnectClicked(close)

	card.Append(btnBox)

	centerBox.Append(card)
	scrim.Append(centerBox)
	win.SetChild(scrim)

	surfaceutil.AddEscapeToClose(win)
	win.SetVisible(true)
}

// PasswordDialog shows an M3-styled dialog with a password entry field as a
// layer-shell overlay.
func PasswordDialog(parent *gtk.ApplicationWindow, title, message, placeholder string, onConfirm func(password string)) {
	win := layershell.NewWindow(parent.Application(), layershell.WindowConfig{
		Name:          "snry-m3-password-dialog",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeOnDemand,
		ExclusiveZone: -1,
		Namespace:     "snry-m3-dialog",
	})

	close := func() { win.SetVisible(false) }

	// Scrim background.
	scrim := gtk.NewBox(gtk.OrientationVertical, 0)
	scrim.AddCSSClass("m3-dialog-scrim")
	scrim.SetHExpand(true)
	scrim.SetVExpand(true)
	clickGesture := gtk.NewGestureClick()
	clickGesture.SetButton(1)
	clickGesture.SetPropagationLimit(gtk.LimitNone)
	clickGesture.ConnectReleased(func(_ int, _ float64, _ float64) { close() })
	scrim.AddController(clickGesture)

	// Centered dialog card.
	centerBox := gtk.NewBox(gtk.OrientationVertical, 0)
	centerBox.SetHAlign(gtk.AlignCenter)
	centerBox.SetVAlign(gtk.AlignCenter)
	centerBox.SetVExpand(true)

	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("m3-dialog")

	titleLabel := gtk.NewLabel(title)
	titleLabel.AddCSSClass("m3-dialog-title")
	titleLabel.SetWrap(true)
	titleLabel.SetXAlign(0)
	card.Append(titleLabel)

	if message != "" {
		msgLabel := gtk.NewLabel(message)
		msgLabel.AddCSSClass("m3-dialog-content")
		msgLabel.SetWrap(true)
		msgLabel.SetXAlign(0)
		card.Append(msgLabel)
	}

	// Password entry with show/hide toggle.
	pwdBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	pwdBox.AddCSSClass("m3-password-box")

	entry := gtk.NewEntry()
	entry.AddCSSClass("m3-password-entry")
	entry.SetPlaceholderText(placeholder)
	entry.SetVisibility(false)
	entry.SetHExpand(true)

	eyeBtn := MaterialButton("visibility_off")
	eyeBtn.AddCSSClass("m3-password-eye")
	eyeBtn.ConnectClicked(func() {
		if entry.Visibility() {
			entry.SetVisibility(false)
			eyeBtn.SetChild(MaterialIcon("visibility_off"))
		} else {
			entry.SetVisibility(true)
			eyeBtn.SetChild(MaterialIcon("visibility"))
		}
	})

	pwdBox.Append(entry)
	pwdBox.Append(eyeBtn)
	card.Append(pwdBox)

	btnBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
	btnBox.AddCSSClass("m3-dialog-actions")

	cancelBtn := gtk.NewButtonWithLabel("Cancel")
	cancelBtn.AddCSSClass("m3-dialog-btn")

	connectBtn := gtk.NewButtonWithLabel("Connect")
	connectBtn.AddCSSClass("m3-dialog-btn")
	connectBtn.AddCSSClass("m3-dialog-btn-primary")

	btnBox.Append(cancelBtn)
	btnBox.Append(connectBtn)
	card.Append(btnBox)

	centerBox.Append(card)
	scrim.Append(centerBox)
	win.SetChild(scrim)

	submit := func() {
		password := entry.Text()
		if password == "" {
			return
		}
		close()
		onConfirm(password)
	}

	cancelBtn.ConnectClicked(close)
	connectBtn.ConnectClicked(submit)
	entry.ConnectActivate(submit)

	surfaceutil.AddEscapeToClose(win)
	win.SetVisible(true)
	glib.IdleAdd(func() { entry.GrabFocus() })
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
