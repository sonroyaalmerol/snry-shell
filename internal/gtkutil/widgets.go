package gtkutil

import (
	"fmt"
	"strconv"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// SetupScrollHoverSuppression adds a "scrolling" CSS class to a ScrolledWindow
// while it is actively scrolling, and removes it shortly after. This prevents
// stuck hover effects on touch devices where a finger drag looks like a hover.
func SetupScrollHoverSuppression(scroll *gtk.ScrolledWindow) {
	var scrollTimeout glib.SourceHandle
	vadj := scroll.VAdjustment()

	vadj.ConnectValueChanged(func() {
		scroll.AddCSSClass("scrolling")
		if scrollTimeout != 0 {
			glib.SourceRemove(scrollTimeout)
		}
		scrollTimeout = glib.TimeoutAdd(150, func() bool {
			scroll.RemoveCSSClass("scrolling")
			scrollTimeout = 0
			return false
		})
	})
}

// ScrollPanel creates a ScrolledWindow with optional hover suppression for
// touch devices. Pass suppressHover=true for any panel that has clickable rows
// that could get stuck in hover state after touch scrolling.
func ScrollPanel(child gtk.Widgetter, suppressHover bool, classes ...string) *gtk.ScrolledWindow {
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	for _, c := range classes {
		scroll.AddCSSClass(c)
	}
	scroll.SetChild(child)
	if suppressHover {
		SetupScrollHoverSuppression(scroll)
	}
	return scroll
}

// LabeledRow creates a horizontal row with a left-aligned title+subtitle text
// block and a trailing control widget. This is the standard row primitive used
// throughout quick-settings, control panel, and settings windows.
//
// If subtitle is empty, only the title label is rendered.
func LabeledRow(title, subtitle string, control gtk.Widgetter, classes ...string) *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 16)
	row.AddCSSClass("m3-switch-row")
	for _, c := range classes {
		row.AddCSSClass(c)
	}

	textBox := gtk.NewBox(gtk.OrientationVertical, 4)
	textBox.SetHExpand(true)

	titleLabel := gtk.NewLabel(title)
	titleLabel.AddCSSClass("m3-switch-row-label")
	titleLabel.SetHAlign(gtk.AlignStart)
	textBox.Append(titleLabel)

	if subtitle != "" {
		sub := gtk.NewLabel(subtitle)
		sub.AddCSSClass("m3-switch-row-sublabel")
		sub.SetHAlign(gtk.AlignStart)
		textBox.Append(sub)
	}

	row.Append(textBox)
	if control != nil {
		row.Append(control)
	}
	return row
}

// SwitchRowFull creates a LabeledRow with an M3 switch, wiring up the callback
// and returning the row so callers can subscribe to state updates.
func SwitchRowFull(title, subtitle string, active bool, onChange func(bool)) (*gtk.Box, *gtk.Switch) {
	sw := M3Switch()
	sw.SetActive(active)
	sw.ConnectStateSet(func(state bool) bool {
		if onChange != nil {
			onChange(state)
		}
		return false
	})
	row := LabeledRow(title, subtitle, sw)
	return row, sw
}

// DropdownRow creates a LabeledRow with a drop-down selector. options is the
// list of string values; current is the initially-selected value. onChange is
// called whenever the selection changes.
func DropdownRow(title, subtitle string, options []string, current string, onChange func(string)) *gtk.Box {
	dropdown := gtk.NewDropDownFromStrings(options)
	dropdown.AddCSSClass("settings-dropdown")
	dropdown.SetCursorFromName("pointer")

	for i, opt := range options {
		if opt == current {
			dropdown.SetSelected(uint(i))
			break
		}
	}
	dropdown.Connect("notify::selected", func() {
		idx := dropdown.Selected()
		if int(idx) < len(options) && onChange != nil {
			onChange(options[idx])
		}
	})

	return LabeledRow(title, subtitle, dropdown)
}

// SpinRow creates a LabeledRow with a text entry that accepts an integer in
// [min, max]. The callback fires on Enter or focus-out. Invalid/out-of-range
// input reverts the entry to the last valid value.
func SpinRow(title, subtitle string, min, max, current int, onChange func(int)) *gtk.Box {
	cur := current
	entry := gtk.NewEntry()
	entry.AddCSSClass("settings-spin-entry")
	entry.SetText(fmt.Sprintf("%d", cur))
	entry.SetMaxWidthChars(4)
	entry.SetHAlign(gtk.AlignEnd)

	apply := func() {
		v, err := strconv.Atoi(entry.Text())
		if err != nil || v < min || v > max {
			entry.SetText(fmt.Sprintf("%d", cur))
			return
		}
		if v != cur {
			cur = v
			if onChange != nil {
				onChange(v)
			}
		}
	}

	entry.ConnectActivate(apply)
	focusCtrl := gtk.NewEventControllerFocus()
	focusCtrl.ConnectLeave(apply)
	entry.AddController(focusCtrl)

	return LabeledRow(title, subtitle, entry)
}

// SettingsSection builds a titled section with a "system-controls" card
// containing the provided child widgets separated by M3 dividers.
func SettingsSection(title string, children ...gtk.Widgetter) *gtk.Box {
	section := gtk.NewBox(gtk.OrientationVertical, 12)
	section.AddCSSClass("settings-page")

	if title != "" {
		lbl := gtk.NewLabel(title)
		lbl.AddCSSClass("settings-label")
		lbl.SetHAlign(gtk.AlignStart)
		section.Append(lbl)
	}

	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("system-controls")

	for i, child := range children {
		if i > 0 {
			card.Append(M3Divider())
		}
		card.Append(child)
	}

	section.Append(card)
	return section
}

// PasswordEntry creates an entry pre-configured for password input: hidden by
// default, with a trailing eye-toggle button to show/hide the value.
// onSubmit is called when the user presses Enter (may be nil).
func PasswordEntry(placeholder string, onSubmit func(string)) (*gtk.Box, *gtk.Entry) {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("m3-password-box")

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

	if onSubmit != nil {
		entry.ConnectActivate(func() { onSubmit(entry.Text()) })
	}

	box.Append(entry)
	box.Append(eyeBtn)
	return box, entry
}
