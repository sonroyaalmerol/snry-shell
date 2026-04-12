package widgets

import (
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

func init() {
	gtk.Init()
}

// asTarget extracts the ConstraintTargetter from a widget.
func asTarget(w gtk.Widgetter) *gtk.ConstraintTarget {
	return &surfaceutil.AsWidget(w).ConstraintTarget
}

// TestQuickToggleColumnsEqualWidth uses ConstraintLayout to enforce
// equal widths regardless of content.
func TestQuickToggleColumnsEqualWidth(t *testing.T) {
	layout := gtk.NewConstraintLayout()

	shortBtn := makeTestToggleBtn("wifi", "WiFi")
	longBtn := makeTestToggleBtn("keyboard", "On-Screen Keyboard")

	strength := int(gtk.ConstraintStrengthRequired)

	layout.AddConstraint(gtk.NewConstraint(
		asTarget(longBtn), gtk.ConstraintAttributeWidth,
		gtk.ConstraintRelationEq,
		asTarget(shortBtn), gtk.ConstraintAttributeWidth,
		1.0, 0.0, strength,
	))

	for _, btn := range []*gtk.Button{shortBtn, longBtn} {
		layout.AddConstraint(gtk.NewConstraintConstant(
			asTarget(btn), gtk.ConstraintAttributeTop, gtk.ConstraintRelationEq, 0, strength))
		layout.AddConstraint(gtk.NewConstraintConstant(
			asTarget(btn), gtk.ConstraintAttributeHeight, gtk.ConstraintRelationGE, 0, strength))
	}

	root := gtk.NewBox(gtk.OrientationHorizontal, 8)
	root.SetLayoutManager(layout)
	root.SetSizeRequest(396, -1)

	win := gtk.NewWindow()
	win.SetDefaultSize(420, 600)
	win.SetChild(root)
	win.Show()

	surfaceutil.AsWidget(root).QueueAllocate()
	surfaceutil.AsWidget(root).SizeAllocate(surfaceutil.AsWidget(root).Allocation(), -1)

	shortW := surfaceutil.AsWidget(shortBtn).Allocation().Width()
	longW := surfaceutil.AsWidget(longBtn).Allocation().Width()

	t.Logf("short=%d long=%d", shortW, longW)

	if shortW != longW {
		t.Errorf("widths unequal: short=%d long=%d", shortW, longW)
	}
}

func makeTestToggleBtn(icon, label string) *gtk.Button {
	btn := gtk.NewButton()
	btn.AddCSSClass("quick-toggle")
	inner := gtk.NewBox(gtk.OrientationHorizontal, 8)
	inner.SetHAlign(gtk.AlignFill)
	iconLbl := gtkutil.MaterialIcon(icon, "quick-toggle-icon")
	textLbl := gtk.NewLabel(label)
	textLbl.AddCSSClass("quick-toggle-label")
	textLbl.SetHAlign(gtk.AlignStart)
	textLbl.SetVAlign(gtk.AlignCenter)
	textLbl.SetXAlign(0)
	textLbl.SetWrap(true)
	inner.Append(iconLbl)
	inner.Append(textLbl)
	btn.SetChild(inner)
	return btn
}
