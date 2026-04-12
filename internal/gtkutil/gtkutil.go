package gtkutil

import (
	"fmt"
	"math"

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
	btn.SetCursorFromName("pointer")
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

// M3IconButton creates a standard M3 icon button with hover state.
func M3IconButton(icon string, classes ...string) *gtk.Button {
	btn := gtk.NewButton()
	btn.AddCSSClass("m3-icon-btn")
	for _, c := range classes {
		btn.AddCSSClass(c)
	}
	btn.SetChild(MaterialIcon(icon))
	btn.SetCursorFromName("pointer")
	return btn
}

// M3FilledButton creates an M3 filled (primary) button with text label.
func M3FilledButton(text string, classes ...string) *gtk.Button {
	btn := gtk.NewButtonWithLabel(text)
	btn.AddCSSClass("m3-filled-btn")
	for _, c := range classes {
		btn.AddCSSClass(c)
	}
	btn.SetCursorFromName("pointer")
	return btn
}

// M3Slider creates a Material Design 3 styled horizontal slider.
// The slider has an enlarged touch target and blocks vertical scroll
// events from propagating to parent ScrolledWindows, which is required
// for reliable touch drag interaction inside scrollable containers.
func M3Slider(min, max, step float64) *gtk.Scale {
	scale := gtk.NewScaleWithRange(gtk.OrientationHorizontal, min, max, step)
	scale.AddCSSClass("m3-scale")
	scale.SetDrawValue(false)
	scale.SetHExpand(true)

	// Block vertical scroll events at capture phase so a parent
	// ScrolledWindow doesn't steal the touch drag for scrolling.
	scrollCtrl := gtk.NewEventControllerScroll(gtk.EventControllerScrollVertical)
	scrollCtrl.SetPropagationPhase(gtk.PhaseCapture)
	scrollCtrl.ConnectScroll(func(_, _ float64) bool { return true })
	scale.AddController(scrollCtrl)

	return scale
}

// M3Divider creates a horizontal separator line.
func M3Divider() *gtk.Separator {
	s := gtk.NewSeparator(gtk.OrientationHorizontal)
	s.AddCSSClass("popup-separator")
	return s
}

// M3Switch creates a Material Design 3 styled switch toggle.
// This uses a custom widget with exact Material 3 dimensions (52x32dp track, 24x24dp thumb).
func M3Switch() *M3CustomSwitch {
	return NewM3CustomSwitch()
}

// SwitchRow creates a labeled row with an M3 switch on the trailing side.
// Prefer SwitchRowFull from widgets.go for new code — it supports a subtitle
// and wires the callback automatically.
func SwitchRow(label string, sw *M3CustomSwitch) *gtk.Box {
	return LabeledRow(label, "", sw)
}

// newDialogBase creates the shared layer-shell overlay window, scrim, card, and
// title label used by all M3 dialog types. Returns (win, card, close).
func newDialogBase(parent *gtk.ApplicationWindow, title string) (*gtk.ApplicationWindow, *gtk.Box, func()) {
	// Use KeyboardModeExclusive so the dialog captures all input
	// This ensures clicks on other shell surfaces (like the bar) dismiss the dialog
	win := surfaceutil.NewFullscreenOverlay(parent.Application(), "snry-m3-dialog", layershell.KeyboardModeExclusive)

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

	centerBox.Append(card)
	scrim.Append(centerBox)
	win.SetChild(scrim)

	surfaceutil.AddEscapeToClose(win)
	win.SetVisible(true)

	return win, card, close
}

// ConfirmDialog shows an M3-styled confirmation dialog as a layer-shell overlay.
// Calls onConfirm if the action button is clicked, or dismisses on outside click / Escape.
func ConfirmDialog(parent *gtk.ApplicationWindow, title, message, action string, onConfirm func()) {
	_, card, close := newDialogBase(parent, title)

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

	cancelBtn.ConnectClicked(close)
	actionBtn.ConnectClicked(func() {
		close()
		onConfirm()
	})
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
	_, card, close := newDialogBase(parent, title)

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
}

// AddPasswordToggle wires btn to toggle entry's password visibility,
// swapping the eye icon accordingly. Call once after creating the entry and button.
func AddPasswordToggle(entry *gtk.Entry, btn *gtk.Button) {
	btn.ConnectClicked(func() {
		if entry.Visibility() {
			entry.SetVisibility(false)
			btn.SetChild(MaterialIcon("visibility_off"))
		} else {
			entry.SetVisibility(true)
			btn.SetChild(MaterialIcon("visibility"))
		}
	})
}

// PasswordDialog shows an M3-styled dialog with a password entry field as a
// layer-shell overlay.
func PasswordDialog(parent *gtk.ApplicationWindow, title, message string, onConfirm func(password string)) {
	win, card, close := newDialogBase(parent, title)

	if message != "" {
		msgLabel := gtk.NewLabel(message)
		msgLabel.AddCSSClass("m3-dialog-content")
		msgLabel.SetWrap(true)
		msgLabel.SetXAlign(0)
		card.Append(msgLabel)
	}

	pwdBox, entry := PasswordEntry(nil)
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

	glib.IdleAdd(func() { entry.GrabFocus() })
	_ = win // window kept alive by GTK after SetVisible(true) in newDialogBase
}

// ErrorDialog shows an M3-styled error dialog as a layer-shell overlay.
func ErrorDialog(parent *gtk.ApplicationWindow, title, message string) {
	_, card, close := newDialogBase(parent, title)

	msgLabel := gtk.NewLabel(message)
	msgLabel.AddCSSClass("m3-dialog-content")
	msgLabel.AddCSSClass("m3-dialog-error-content")
	msgLabel.SetWrap(true)
	msgLabel.SetXAlign(0)
	card.Append(msgLabel)

	btnBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
	btnBox.AddCSSClass("m3-dialog-actions")

	dismissBtn := gtk.NewButtonWithLabel("OK")
	dismissBtn.AddCSSClass("m3-dialog-btn")
	btnBox.Append(dismissBtn)
	card.Append(btnBox)

	dismissBtn.ConnectClicked(close)
}

// dragThreshold is the maximum distance (in pixels) allowed between press and release
// to consider it a click rather than a drag/scroll
const dragThreshold = 15.0

// ClaimedClick adds a left-click gesture that claims the sequence on press,
// preventing the default button activation. If onRelease is non-nil it is
// called on release. Returns the gesture for additional handler attachment.
// This version includes drag detection - if the cursor moves more than
// dragThreshold pixels between press and release, the click is not triggered.
func ClaimedClick(w *gtk.Widget, onRelease func()) *gtk.GestureClick {
	click := gtk.NewGestureClick()
	click.SetButton(1)

	var startX, startY float64

	click.ConnectPressed(func(_ int, x, y float64) {
		startX = x
		startY = y
	})

	if onRelease != nil {
		click.ConnectReleased(func(_ int, x, y float64) {
			// Calculate distance moved
			dx := x - startX
			dy := y - startY
			distance := math.Sqrt(dx*dx + dy*dy)

			// Only trigger click if movement is below threshold
			if distance < dragThreshold {
				onRelease()
			}
		})
	}

	w.AddController(click)
	return click
}

// SectionHeader creates a clickable header for a collapsible section.
func SectionHeader(title string, count int, revealer *gtk.Revealer, onScan func()) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, 8)
	box.AddCSSClass("conn-section-header")
	box.SetCursorFromName("pointer")

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
	ClaimedClick(&box.Widget, func() {
		expanded = !expanded
		revealer.SetRevealChild(expanded)
		if expanded {
			arrow.SetText("expand_more")
		} else {
			arrow.SetText("expand_less")
		}
	})

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
