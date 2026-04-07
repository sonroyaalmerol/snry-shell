package gtkutil

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// ── Buttons ──────────────────────────────────────────────────────────────────

// M3TextButton creates an M3 text (low-emphasis) button.
func M3TextButton(text string, classes ...string) *gtk.Button {
	btn := gtk.NewButtonWithLabel(text)
	btn.AddCSSClass("m3-text-btn")
	for _, c := range classes {
		btn.AddCSSClass(c)
	}
	btn.SetCursorFromName("pointer")
	return btn
}

// M3OutlinedButton creates an M3 outlined (medium-emphasis) button.
func M3OutlinedButton(text string, classes ...string) *gtk.Button {
	btn := gtk.NewButtonWithLabel(text)
	btn.AddCSSClass("m3-outlined-btn")
	for _, c := range classes {
		btn.AddCSSClass(c)
	}
	btn.SetCursorFromName("pointer")
	return btn
}

// M3TonalButton creates an M3 tonal (secondary filled) button.
func M3TonalButton(text string, classes ...string) *gtk.Button {
	btn := gtk.NewButtonWithLabel(text)
	btn.AddCSSClass("m3-tonal-btn")
	for _, c := range classes {
		btn.AddCSSClass(c)
	}
	btn.SetCursorFromName("pointer")
	return btn
}

// ── Cards ─────────────────────────────────────────────────────────────────────

// M3Card returns an elevated card container box. variant is one of "elevated"
// (default), "filled", or "outlined".
func M3Card(variant string, classes ...string) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	switch variant {
	case "filled":
		box.AddCSSClass("m3-card-filled")
	case "outlined":
		box.AddCSSClass("m3-card-outlined")
	default:
		box.AddCSSClass("m3-card")
	}
	for _, c := range classes {
		box.AddCSSClass(c)
	}
	return box
}

// ── Chips ─────────────────────────────────────────────────────────────────────

// M3AssistChip creates a non-interactive assist chip (icon + label).
// Pass an empty icon to omit the icon.
func M3AssistChip(icon, text string, classes ...string) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, 6)
	box.AddCSSClass("m3-assist-chip")
	for _, c := range classes {
		box.AddCSSClass(c)
	}
	if icon != "" {
		box.Append(MaterialIcon(icon))
	}
	if text != "" {
		lbl := gtk.NewLabel(text)
		box.Append(lbl)
	}
	return box
}

// M3FilterChip creates an interactive filter chip (toggle button style).
func M3FilterChip(icon, text string, active bool, onChange func(bool), classes ...string) *gtk.ToggleButton {
	tb := gtk.NewToggleButton()
	tb.AddCSSClass("m3-filter-chip")
	for _, c := range classes {
		tb.AddCSSClass(c)
	}
	tb.SetActive(active)
	tb.SetCursorFromName("pointer")

	inner := gtk.NewBox(gtk.OrientationHorizontal, 6)
	if icon != "" {
		inner.Append(MaterialIcon(icon))
	}
	if text != "" {
		lbl := gtk.NewLabel(text)
		inner.Append(lbl)
	}
	tb.SetChild(inner)

	if onChange != nil {
		tb.ConnectToggled(func() { onChange(tb.Active()) })
	}
	return tb
}

// M3InputChip creates a dismissible input chip (label + remove button).
func M3InputChip(text string, onRemove func(), classes ...string) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.AddCSSClass("m3-input-chip")
	for _, c := range classes {
		box.AddCSSClass(c)
	}
	lbl := gtk.NewLabel(text)
	box.Append(lbl)
	if onRemove != nil {
		removeBtn := MaterialButton("close")
		removeBtn.AddCSSClass("m3-icon-btn")
		removeBtn.ConnectClicked(onRemove)
		box.Append(removeBtn)
	}
	return box
}

// ── Badge ─────────────────────────────────────────────────────────────────────

// M3Badge creates a small badge label for counts or status dots.
// Pass count <= 0 to render a small dot badge (no text).
func M3Badge(count int, classes ...string) *gtk.Label {
	lbl := gtk.NewLabel("")
	lbl.AddCSSClass("m3-badge")
	for _, c := range classes {
		lbl.AddCSSClass(c)
	}
	if count <= 0 {
		lbl.AddCSSClass("m3-badge-small")
	} else {
		if count > 99 {
			lbl.SetText("99+")
		} else {
			lbl.SetText(fmt.Sprintf("%d", count))
		}
	}
	return lbl
}

// ── Loading & Progress ────────────────────────────────────────────────────────

// M3Spinner returns a MaterialIcon configured as an animated spinner.
// Use the "spinner-icon" CSS class (defined in style.css) to apply rotation.
func M3Spinner(classes ...string) *gtk.Label {
	icon := MaterialIcon("progress_activity", "spinner-icon")
	icon.AddCSSClass("m3-spinner")
	for _, c := range classes {
		icon.AddCSSClass(c)
	}
	return icon
}

// M3ProgressBar creates a linear determinate/indeterminate progress bar.
// Call SetFraction to set progress (0–1); call SetPulseStep + Pulse for
// indeterminate mode.
func M3ProgressBar(classes ...string) *gtk.ProgressBar {
	bar := gtk.NewProgressBar()
	bar.AddCSSClass("m3-progress-bar")
	bar.SetShowText(false)
	for _, c := range classes {
		bar.AddCSSClass(c)
	}
	return bar
}

// ── Text fields ───────────────────────────────────────────────────────────────

// M3TextField creates a single-line text entry with the M3 outlined text field style.
// Returns the field widget and the entry widget.
func M3TextField(classes ...string) (*M3OutlinedTextField, *gtk.Entry) {
	field := NewM3OutlinedTextField()
	for _, c := range classes {
		field.AddCSSClass(c)
	}
	return field, field.Entry()
}

// M3PasswordField creates a password entry with the M3 outlined text field style.
// Returns the field widget and the entry widget.
func M3PasswordField(classes ...string) (*M3OutlinedTextField, *gtk.Entry) {
	field := NewM3OutlinedPasswordField()
	for _, c := range classes {
		field.AddCSSClass(c)
	}
	return field, field.Entry()
}

// ── Composite helpers ─────────────────────────────────────────────────────────

// IconLabel creates a horizontal box containing a MaterialIcon and a text
// label side by side. iconClasses are applied to the icon label; textClasses
// to the text label.
func IconLabel(icon, text string, iconClasses, textClasses []string) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, 8)
	ic := MaterialIcon(icon, iconClasses...)
	lbl := gtk.NewLabel(text)
	for _, c := range textClasses {
		lbl.AddCSSClass(c)
	}
	box.Append(ic)
	box.Append(lbl)
	return box
}

// SliderRow creates a grid row with an icon, a label, and an M3Slider.
// Returns the three widgets individually so callers can subscribe to value
// changes on the slider.
func SliderRow(icon, label string, min, max, step float64, iconClass, labelClass string) (*gtk.Box, *gtk.Label, *gtk.Scale) {
	box := gtk.NewBox(gtk.OrientationHorizontal, 8)
	box.SetHExpand(true)

	ic := MaterialIcon(icon)
	if iconClass != "" {
		ic.AddCSSClass(iconClass)
	}
	ic.SetVAlign(gtk.AlignCenter)

	lbl := gtk.NewLabel(label)
	if labelClass != "" {
		lbl.AddCSSClass(labelClass)
	}
	lbl.SetHAlign(gtk.AlignStart)
	lbl.SetVAlign(gtk.AlignCenter)

	scale := M3Slider(min, max, step)

	box.Append(ic)
	box.Append(lbl)
	box.Append(scale)
	return box, lbl, scale
}

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
func SwitchRowFull(title, subtitle string, active bool, onChange func(bool)) (*gtk.Box, *M3CustomSwitch) {
	sw := M3Switch()
	sw.SetActive(active)
	sw.ConnectStateSet(func(state bool) bool {
		if onChange != nil {
			onChange(state)
		}
		return true
	})
	row := LabeledRow(title, subtitle, sw)
	return row, sw
}

// DropdownRow creates a LabeledRow with a drop-down selector. options is the
// list of string values; current is the initially-selected value. onChange is
// called whenever the selection changes.
func DropdownRow(title, subtitle string, options []string, current string, onChange func(string)) *gtk.Box {
	dropdown := NewM3Dropdown(options)
	for i, opt := range options {
		if opt == current {
			dropdown.SetSelected(i)
			break
		}
	}
	dropdown.ConnectSelected(func(idx int) {
		if idx >= 0 && idx < len(options) && onChange != nil {
			onChange(options[idx])
		}
	})

	return LabeledRow(title, subtitle, dropdown)
}

// SpinRow creates a LabeledRow with a number field that accepts an integer in
// [min, max]. The callback fires on value change.
func SpinRow(title, subtitle string, min, max, current int, onChange func(int)) *gtk.Box {
	numberField := NewM3NumberField(min, max, current)
	numberField.ConnectChanged(func(value int) {
		if onChange != nil {
			onChange(value)
		}
	})

	return LabeledRow(title, subtitle, numberField)
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
func PasswordEntry(onSubmit func(string)) (*gtk.Box, *gtk.Entry) {
	// Create the M3 outlined password field
	field := NewM3OutlinedPasswordField()

	// Add eye toggle button
	eyeBtn := MaterialButton("visibility_off")
	eyeBtn.AddCSSClass("m3-password-eye")
	eyeBtn.SetVAlign(gtk.AlignCenter)
	AddPasswordToggle(field.Entry(), eyeBtn)

	// Create a box to hold the field and button
	box := gtk.NewBox(gtk.OrientationHorizontal, 8)
	box.SetHExpand(true)
	field.SetHExpand(true)
	box.Append(field)
	box.Append(eyeBtn)

	if onSubmit != nil {
		field.ConnectActivate(func(text string) { onSubmit(text) })
	}

	return box, field.Entry()
}
