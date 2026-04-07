package gtkutil

import (
	"strconv"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// ── Material 3 Outlined Text Field ─────────────────────────────────────────────

// M3OutlinedTextField is a Material Design 3 outlined text field (no label inside).
type M3OutlinedTextField struct {
	*gtk.Box
	entry    *gtk.Entry
	hasFocus bool
}

// NewM3OutlinedTextField creates a new Material 3 outlined text field.
// Label should be added separately by the caller for consistency with other components.
func NewM3OutlinedTextField() *M3OutlinedTextField {
	field := &M3OutlinedTextField{
		Box: gtk.NewBox(gtk.OrientationVertical, 0),
	}

	field.AddCSSClass("m3-outlined-field")

	// Create the border container
	inputContainer := gtk.NewBox(gtk.OrientationVertical, 0)
	inputContainer.AddCSSClass("m3-outlined-input-border")

	// Create text entry
	field.entry = gtk.NewEntry()
	field.entry.AddCSSClass("m3-outlined-input")
	field.entry.SetHasFrame(false)

	inputContainer.Append(field.entry)
	field.Append(inputContainer)

	// Handle focus
	focusCtrl := gtk.NewEventControllerFocus()
	focusCtrl.ConnectEnter(func() {
		field.hasFocus = true
		field.AddCSSClass("focused")
	})
	focusCtrl.ConnectLeave(func() {
		field.hasFocus = false
		field.RemoveCSSClass("focused")
	})
	field.entry.AddController(focusCtrl)

	return field
}

// NewM3OutlinedPasswordField creates a password variant.
func NewM3OutlinedPasswordField() *M3OutlinedTextField {
	field := NewM3OutlinedTextField()
	field.entry.SetVisibility(false)
	field.entry.SetInputPurpose(gtk.InputPurposePassword)
	return field
}

// Entry returns the underlying entry widget.
func (f *M3OutlinedTextField) Entry() *gtk.Entry {
	return f.entry
}

// Text returns the current text value.
func (f *M3OutlinedTextField) Text() string {
	return f.entry.Text()
}

// SetText sets the text value.
func (f *M3OutlinedTextField) SetText(text string) {
	f.entry.SetText(text)
}

// SetSensitive enables/disables the field.
func (f *M3OutlinedTextField) SetSensitive(sensitive bool) {
	f.Box.SetSensitive(sensitive)
	f.entry.SetSensitive(sensitive)
}

// ConnectChanged connects a callback for text changes.
func (f *M3OutlinedTextField) ConnectChanged(callback func(string)) {
	f.entry.ConnectChanged(func() {
		callback(f.entry.Text())
	})
}

// ConnectActivate connects a callback for Enter key.
func (f *M3OutlinedTextField) ConnectActivate(callback func(string)) {
	f.entry.ConnectActivate(func() {
		callback(f.entry.Text())
	})
}

// ── Material 3 Dropdown ────────────────────────────────────────────────────────

// M3Dropdown is a Material Design 3 dropdown menu.
type M3Dropdown struct {
	*gtk.DropDown
	onSelected func(int)
}

// NewM3Dropdown creates a new Material 3 dropdown.
func NewM3Dropdown(items []string) *M3Dropdown {
	dd := &M3Dropdown{
		DropDown: gtk.NewDropDownFromStrings(items),
	}

	dd.AddCSSClass("m3-dropdown")

	if len(items) > 0 {
		dd.SetSelected(0)
	}

	dd.Connect("notify::selected", func() {
		idx := int(dd.Selected())
		if dd.onSelected != nil {
			dd.onSelected(idx)
		}
	})

	return dd
}

// ConnectSelected connects a callback for selection changes.
func (dd *M3Dropdown) ConnectSelected(callback func(int)) {
	dd.onSelected = callback
}

// SetSelected sets the selected index.
func (dd *M3Dropdown) SetSelected(index int) {
	if index >= 0 {
		dd.DropDown.SetSelected(uint(index))
	}
}

// Selected returns the currently selected index.
func (dd *M3Dropdown) Selected() int {
	return int(dd.DropDown.Selected())
}

// ── Material 3 Number Field ────────────────────────────────────────────────────

// M3NumberField is a Material Design 3 number input field with +/- buttons.
type M3NumberField struct {
	*gtk.Box
	entry  *gtk.Entry
	decBtn *gtk.Button
	incBtn *gtk.Button

	value int
	min   int
	max   int

	onChange func(int)
}

// NewM3NumberField creates a new Material 3 number field.
func NewM3NumberField(min, max, initial int) *M3NumberField {
	field := &M3NumberField{
		Box:   gtk.NewBox(gtk.OrientationHorizontal, 0),
		value: initial,
		min:   min,
		max:   max,
	}

	field.AddCSSClass("m3-number-field")

	// Decrement button
	field.decBtn = gtk.NewButton()
	field.decBtn.AddCSSClass("m3-number-field-btn")
	field.decBtn.SetLabel("−")
	field.decBtn.ConnectClicked(func() {
		field.SetValue(field.value - 1)
	})

	// Entry
	field.entry = gtk.NewEntry()
	field.entry.AddCSSClass("m3-number-field-input")
	field.entry.SetText(strconv.Itoa(initial))
	field.entry.SetWidthChars(4)
	field.entry.SetMaxWidthChars(4)
	field.entry.SetAlignment(0.5)

	// Increment button
	field.incBtn = gtk.NewButton()
	field.incBtn.AddCSSClass("m3-number-field-btn")
	field.incBtn.SetLabel("+")
	field.incBtn.ConnectClicked(func() {
		field.SetValue(field.value + 1)
	})

	// Text entry handling
	field.entry.ConnectChanged(func() {
		text := strings.TrimSpace(field.entry.Text())
		if text == "" {
			return
		}
		v, err := strconv.Atoi(text)
		if err != nil {
			field.entry.SetText(strconv.Itoa(field.value))
			return
		}
		if v >= field.min && v <= field.max {
			if v != field.value {
				field.value = v
				if field.onChange != nil {
					field.onChange(field.value)
				}
			}
		}
	})

	field.entry.ConnectActivate(func() {
		text := strings.TrimSpace(field.entry.Text())
		if text == "" {
			field.SetValue(field.min)
			return
		}
		v, err := strconv.Atoi(text)
		if err != nil {
			field.entry.SetText(strconv.Itoa(field.value))
			return
		}
		field.SetValue(v)
	})

	field.Append(field.decBtn)
	field.Append(field.entry)
	field.Append(field.incBtn)

	// Focus out handling
	focusCtrl := gtk.NewEventControllerFocus()
	focusCtrl.ConnectLeave(func() {
		text := strings.TrimSpace(field.entry.Text())
		if text == "" {
			field.SetValue(field.min)
			return
		}
		v, err := strconv.Atoi(text)
		if err != nil {
			field.entry.SetText(strconv.Itoa(field.value))
			return
		}
		field.SetValue(v)
	})
	field.entry.AddController(focusCtrl)

	field.updateButtonStates()

	return field
}

// SetValue sets the value and updates the field.
func (f *M3NumberField) SetValue(value int) {
	oldValue := f.value
	f.value = f.clamp(value)
	f.entry.SetText(strconv.Itoa(f.value))
	f.updateButtonStates()

	if oldValue != f.value && f.onChange != nil {
		f.onChange(f.value)
	}
}

// Value returns the current value.
func (f *M3NumberField) Value() int {
	return f.value
}

// SetSensitive enables/disables the field.
func (f *M3NumberField) SetSensitive(sensitive bool) {
	f.Box.SetSensitive(sensitive)
	f.entry.SetSensitive(sensitive)
	f.decBtn.SetSensitive(sensitive)
	f.incBtn.SetSensitive(sensitive)
}

// ConnectChanged connects a callback for value changes.
func (f *M3NumberField) ConnectChanged(callback func(int)) {
	f.onChange = callback
}

// updateButtonStates updates the sensitivity of +/- buttons.
func (f *M3NumberField) updateButtonStates() {
	f.decBtn.SetSensitive(f.value > f.min)
	f.incBtn.SetSensitive(f.value < f.max)
}

// clamp ensures value is within min/max bounds.
func (f *M3NumberField) clamp(v int) int {
	if v < f.min {
		return f.min
	}
	if v > f.max {
		return f.max
	}
	return v
}
