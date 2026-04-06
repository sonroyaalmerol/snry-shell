package controlpanel

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
)

// shellConfigProvider implements ConfigProvider for snry-shell settings
type shellConfigProvider struct {
	cfg *settings.Config
}

func newShellConfigProvider(cfg *settings.Config) *shellConfigProvider {
	return &shellConfigProvider{cfg: cfg}
}

func (s *shellConfigProvider) Name() string {
	return "Shell"
}

func (s *shellConfigProvider) Icon() string {
	return "tune"
}

func (s *shellConfigProvider) Load() error {
	if cfg, err := settings.Load(); err == nil {
		*s.cfg = cfg
	}
	return nil
}

func (s *shellConfigProvider) Save() error {
	return settings.Save(*s.cfg)
}

func (s *shellConfigProvider) BuildWidget() gtk.Widgetter {
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("popup-scroll")

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("settings-stack")

	// Settings sections
	box.Append(s.buildAppearanceSection())
	box.Append(s.buildBehaviorSection())

	scroll.SetChild(box)
	return scroll
}

func (s *shellConfigProvider) buildAppearanceSection() gtk.Widgetter {
	section := gtk.NewBox(gtk.OrientationVertical, 12)
	section.AddCSSClass("settings-page")

	// Section title
	title := gtk.NewLabel("Appearance")
	title.AddCSSClass("settings-label")
	title.SetHAlign(gtk.AlignStart)
	section.Append(title)

	// Card container using system-controls style
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("system-controls")

	// Dark mode toggle - use m3-switch style
	darkModeRow := s.buildSwitchRow("Dark Mode", "Use dark theme", s.cfg.DarkMode, func(active bool) {
		s.cfg.DarkMode = active
		s.Save()
	})
	card.Append(darkModeRow)

	// Separator
	card.Append(gtkutil.M3Divider())

	// Bar position dropdown
	barPosRow := s.buildDropdownRow("Bar Position", "Position of the status bar", []string{"top", "bottom"}, s.cfg.BarPosition, func(value string) {
		s.cfg.BarPosition = value
		s.Save()
	})
	card.Append(barPosRow)

	// Separator
	card.Append(gtkutil.M3Divider())

	// Font scale using m3-scale slider
	fontScaleRow := s.buildSliderRow("Font Scale", "Adjust text size", 0.5, 2.0, 0.1, s.cfg.FontScale, func(value float64) {
		s.cfg.FontScale = value
		s.Save()
	})
	card.Append(fontScaleRow)

	section.Append(card)
	return section
}

func (s *shellConfigProvider) buildBehaviorSection() gtk.Widgetter {
	section := gtk.NewBox(gtk.OrientationVertical, 12)
	section.AddCSSClass("settings-page")
	section.SetMarginTop(24)

	// Section title
	title := gtk.NewLabel("Behavior")
	title.AddCSSClass("settings-label")
	title.SetHAlign(gtk.AlignStart)
	section.Append(title)

	// Card container
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("system-controls")

	// Do Not Disturb toggle
	dndRow := s.buildSwitchRow("Do Not Disturb", "Silence notifications", s.cfg.DoNotDisturb, func(active bool) {
		s.cfg.DoNotDisturb = active
		s.Save()
	})
	card.Append(dndRow)

	// Separator
	card.Append(gtkutil.M3Divider())

	// Input mode dropdown
	inputModeRow := s.buildDropdownRow("Input Mode", "Touch input handling", []string{"auto", "tablet", "desktop"}, s.cfg.InputMode, func(value string) {
		s.cfg.InputMode = value
		s.Save()
	})
	card.Append(inputModeRow)

	section.Append(card)
	return section
}

func (s *shellConfigProvider) buildSwitchRow(title, subtitle string, active bool, callback func(bool)) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 16)
	row.AddCSSClass("m3-switch-row")

	// Text content
	textBox := gtk.NewBox(gtk.OrientationVertical, 4)
	textBox.SetHExpand(true)

	titleLabel := gtk.NewLabel(title)
	titleLabel.AddCSSClass("m3-switch-row-label")
	titleLabel.SetHAlign(gtk.AlignStart)
	textBox.Append(titleLabel)

	if subtitle != "" {
		subtitleLabel := gtk.NewLabel(subtitle)
		subtitleLabel.AddCSSClass("m3-switch-row-sublabel")
		subtitleLabel.SetHAlign(gtk.AlignStart)
		textBox.Append(subtitleLabel)
	}

	row.Append(textBox)

	// Use m3-switch class from shell
	switchBtn := gtk.NewSwitch()
	switchBtn.AddCSSClass("m3-switch")
	switchBtn.SetActive(active)
	switchBtn.ConnectStateSet(func(state bool) bool {
		callback(state)
		return false
	})
	row.Append(switchBtn)

	return row
}

func (s *shellConfigProvider) buildDropdownRow(title, subtitle string, options []string, current string, callback func(string)) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 16)
	row.AddCSSClass("m3-switch-row")

	// Text content
	textBox := gtk.NewBox(gtk.OrientationVertical, 4)
	textBox.SetHExpand(true)

	titleLabel := gtk.NewLabel(title)
	titleLabel.AddCSSClass("m3-switch-row-label")
	titleLabel.SetHAlign(gtk.AlignStart)
	textBox.Append(titleLabel)

	if subtitle != "" {
		subtitleLabel := gtk.NewLabel(subtitle)
		subtitleLabel.AddCSSClass("m3-switch-row-sublabel")
		subtitleLabel.SetHAlign(gtk.AlignStart)
		textBox.Append(subtitleLabel)
	}

	row.Append(textBox)

	// Dropdown with settings-dropdown style
	dropdown := gtk.NewDropDownFromStrings(options)
	dropdown.AddCSSClass("settings-dropdown")

	// Set current value
	for i, opt := range options {
		if opt == current {
			dropdown.SetSelected(uint(i))
			break
		}
	}

	dropdown.Connect("notify::selected", func() {
		idx := dropdown.Selected()
		if idx < uint(len(options)) {
			callback(options[idx])
		}
	})

	row.Append(dropdown)

	return row
}

func (s *shellConfigProvider) buildSliderRow(title, subtitle string, min, max, step, current float64, callback func(float64)) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationVertical, 12)
	row.AddCSSClass("m3-switch-row")

	// Header with title and value
	header := gtk.NewBox(gtk.OrientationHorizontal, 16)

	titleLabel := gtk.NewLabel(title)
	titleLabel.AddCSSClass("m3-switch-row-label")
	titleLabel.SetHAlign(gtk.AlignStart)
	header.Append(titleLabel)

	valueLabel := gtk.NewLabel(fmt.Sprintf("%.1f", current))
	valueLabel.AddCSSClass("quick-slider-label")
	valueLabel.SetHAlign(gtk.AlignEnd)
	valueLabel.SetHExpand(true)
	header.Append(valueLabel)

	row.Append(header)

	if subtitle != "" {
		subtitleLabel := gtk.NewLabel(subtitle)
		subtitleLabel.AddCSSClass("m3-switch-row-sublabel")
		subtitleLabel.SetHAlign(gtk.AlignStart)
		row.Append(subtitleLabel)
	}

	// Use M3 slider from shell
	slider := gtkutil.M3Slider(min, max, step)
	slider.SetValue(current)
	slider.SetHExpand(true)

	slider.ConnectValueChanged(func() {
		value := slider.Value()
		valueLabel.SetText(fmt.Sprintf("%.1f", value))
		callback(value)
	})

	row.Append(slider)

	return row
}
