// Package settings provides the settings panel surface.
package settings

import (
	"log"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	appsettings "github.com/sonroyaalmerol/snry-shell/internal/settings"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

// Settings is a settings panel overlay.
type Settings struct {
	win *gtk.ApplicationWindow
	bus *bus.Bus
	cfg appsettings.Config
}

func New(app *gtk.Application, b *bus.Bus) *Settings {
	win := layershell.NewWindow(app, layershell.WindowConfig{
		Name:          "snry-settings",
		Layer:         layershell.LayerOverlay,
		Anchors:       layershell.FullscreenAnchors(),
		KeyboardMode:  layershell.KeyboardModeExclusive,
		ExclusiveZone: -1,
		Namespace:     "snry-settings",
	})

	cfg, _ := appsettings.Load()
	s := &Settings{win: win, bus: b, cfg: cfg}
	s.build()

	surfaceutil.AddToggleOnWithFocus(b, win, "toggle-settings")

	surfaceutil.AddEscapeToClose(win)
	win.SetVisible(false)
	return s
}

func (s *Settings) build() {
	root := gtk.NewBox(gtk.OrientationHorizontal, 0)
	root.AddCSSClass("settings-panel")

	// Navigation sidebar.
	nav := gtk.NewBox(gtk.OrientationVertical, 0)
	nav.AddCSSClass("settings-nav")

	appearanceBtn := gtk.NewButton()
	appearanceBtn.SetCursorFromName("pointer")
	appearanceBtn.AddCSSClass("settings-nav-item")
	appearanceLabel := gtk.NewLabel("Appearance")
	appearanceLabel.AddCSSClass("settings-nav-label")
	appearanceBtn.SetChild(appearanceLabel)

	barBtn := gtk.NewButton()
	barBtn.SetCursorFromName("pointer")
	barBtn.AddCSSClass("settings-nav-item")
	barLabel := gtk.NewLabel("Bar")
	barLabel.AddCSSClass("settings-nav-label")
	barBtn.SetChild(barLabel)

	nav.Append(appearanceBtn)
	nav.Append(barBtn)
	root.Append(nav)

	// Content area.
	stack := gtk.NewStack()
	stack.AddCSSClass("settings-stack")
	stack.SetHExpand(true)
	stack.SetTransitionType(gtk.StackTransitionTypeSlideLeftRight)

	// Appearance page.
	appearancePage := s.buildAppearancePage()
	stack.AddTitled(appearancePage, "appearance", "Appearance")

	// Bar page.
	barPage := s.buildBarPage()
	stack.AddTitled(barPage, "bar", "Bar")

	root.Append(stack)
	s.win.SetChild(root)

	// Wire navigation.
	appearanceBtn.ConnectClicked(func() {
		stack.SetVisibleChildName("appearance")
	})
	barBtn.ConnectClicked(func() {
		stack.SetVisibleChildName("bar")
	})
}

func (s *Settings) buildAppearancePage() gtk.Widgetter {
	page := gtk.NewBox(gtk.OrientationVertical, 12)
	page.AddCSSClass("settings-page")

	// Dark mode toggle.
	darkRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	darkRow.AddCSSClass("settings-row")

	darkLabel := gtk.NewLabel("Dark Mode")
	darkLabel.AddCSSClass("settings-label")
	darkLabel.SetHExpand(true)
	darkLabel.SetHAlign(gtk.AlignStart)

	darkSwitch := gtk.NewSwitch()
	darkSwitch.SetActive(s.cfg.DarkMode)
	darkSwitch.ConnectStateSet(func(state bool) bool {
		s.cfg.DarkMode = state
		s.save()
		return false
	})

	darkRow.Append(darkLabel)
	darkRow.Append(darkSwitch)
	page.Append(darkRow)

	// Font scale slider.
	fontRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	fontRow.AddCSSClass("settings-row")

	fontLabel := gtk.NewLabel("Font Scale")
	fontLabel.AddCSSClass("settings-label")
	fontLabel.SetHAlign(gtk.AlignStart)

	fontScale := gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0.5, 2.0, 0.1)
	fontScale.AddCSSClass("settings-scale")
	fontScale.AddCSSClass("m3-scale")
	fontScale.SetDrawValue(true)
	fontScale.SetHExpand(true)
	fontScale.SetValue(s.cfg.FontScale)
	fontScale.ConnectChangeValue(func(_ gtk.ScrollType, value float64) bool {
		s.cfg.FontScale = value
		s.save()
		return false
	})

	fontRow.Append(fontLabel)
	fontRow.Append(fontScale)
	page.Append(fontRow)

	return page
}

func (s *Settings) buildBarPage() gtk.Widgetter {
	page := gtk.NewBox(gtk.OrientationVertical, 12)
	page.AddCSSClass("settings-page")

	// Bar position dropdown.
	posRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	posRow.AddCSSClass("settings-row")

	posLabel := gtk.NewLabel("Position")
	posLabel.AddCSSClass("settings-label")
	posLabel.SetHAlign(gtk.AlignStart)

	posDrop := gtk.NewDropDownFromStrings([]string{"top", "bottom"})
	posDrop.AddCSSClass("settings-dropdown")
	posDrop.SetHExpand(true)

	// Select current value.
	for i, v := range []string{"top", "bottom"} {
		if v == s.cfg.BarPosition {
			posDrop.SetSelected(uint(i))
			break
		}
	}

	posDrop.ConnectActivate(func() {
		idx := posDrop.Selected()
		if idx >= 0 && idx < 2 {
			options := []string{"top", "bottom"}
			s.cfg.BarPosition = options[idx]
			s.save()
		}
	})

	posRow.Append(posLabel)
	posRow.Append(posDrop)
	page.Append(posRow)

	return page
}

func (s *Settings) save() {
	if err := appsettings.Save(s.cfg); err != nil {
		log.Printf("settings save: %v", err)
		gtkutil.ErrorDialog(s.win, "Save failed", "Could not save settings.")
	}
}
