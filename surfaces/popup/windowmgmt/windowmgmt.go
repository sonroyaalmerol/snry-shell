package windowmgmt

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/layershell"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
	"github.com/sonroyaalmerol/snry-shell/internal/surfaceutil"
)

const (
	panelMargin   = 12
	panelWidth    = 280
	maxWorkspaces = 10
)

// WindowMgmt is a popup for touch-friendly window management actions.
type WindowMgmt struct {
	win         *gtk.ApplicationWindow
	bus         *bus.Bus
	refs        *servicerefs.ServiceRefs
	trigger     gtk.Widgetter
	monitor     *gdk.Monitor
	root        *gtk.Box
	wsRows      []*gtk.Box
	activeWS    int
	capturedWin string // Address of the window captured when popup opened
	isOpen      bool
}

// New creates and hides the window management popup anchored to the given trigger widget.
func New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs, trigger gtk.Widgetter) *WindowMgmt {
	win, _, root := surfaceutil.NewPopupPanel(app, b, surfaceutil.PopupPanelConfig{
		Name:      "snry-windowmgmt",
		Namespace: "snry-windowmgmt",
		CloseOn:   []string{"toggle-notif-center", "toggle-wifi", "toggle-bluetooth", "toggle-calendar", "toggle-overview"},
		Align:     gtk.AlignStart,
	})

	w := &WindowMgmt{win: win, bus: b, refs: refs, trigger: trigger, root: root}

	panel := gtk.NewBox(gtk.OrientationVertical, 0)
	panel.AddCSSClass("popup-panel")
	panel.SetMarginStart(panelMargin)
	panel.SetMarginEnd(panelMargin)
	panel.SetSizeRequest(panelWidth, -1)

	panel.Append(buildContent(b, refs, w))

	root.Append(panel)

	b.Subscribe(bus.TopicPopupTrigger, func(e bus.Event) {
		pt, ok := e.Data.(surfaceutil.PopupTrigger)
		if !ok {
			return
		}
		if pt.Action == "toggle-windowmgmt" {
			w.trigger = pt.Trigger
			w.monitor = pt.Monitor
		}
	})

	b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		if e.Data == "toggle-windowmgmt" {
			glib.IdleAdd(func() { w.Toggle() })
		}
	})

	b.Subscribe(bus.TopicWorkspaces, func(e bus.Event) {
		ws := e.Data.(state.Workspace)
		if ws.Active {
			w.activeWS = ws.ID
			glib.IdleAdd(w.highlightActiveWorkspace)
		}
	})

	// Listen for active window changes but only update if popup is not open
	b.Subscribe(bus.TopicActiveWindow, func(e bus.Event) {
		if w.isOpen {
			// Don't update - keep showing captured window
			return
		}
	})

	return w
}

func (w *WindowMgmt) Toggle() {
	if w.win.Visible() {
		w.win.SetVisible(false)
		w.isOpen = false
		w.capturedWin = ""
	} else {
		// Capture the current window before opening
		if w.refs.Hyprland != nil {
			if win, err := w.refs.Hyprland.ActiveWindow(); err == nil && win.Address != "" {
				w.capturedWin = win.Address
			}
		}
		w.isOpen = true
		if w.monitor != nil {
			layershell.SetMonitor(w.win, w.monitor)
		}
		surfaceutil.PositionUnderTrigger(w.root, w.trigger, panelWidth, panelMargin, w.monitor)
		w.win.SetVisible(true)
	}
}

func (w *WindowMgmt) highlightActiveWorkspace() {
	for i, row := range w.wsRows {
		wsID := i + 1
		if wsID == w.activeWS {
			row.AddCSSClass("conn-row-connected")
		} else {
			row.RemoveCSSClass("conn-row-connected")
		}
	}
}

func buildContent(b *bus.Bus, refs *servicerefs.ServiceRefs, w *WindowMgmt) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("conn-widget")

	// Action rows - use captured window address.
	actions := []struct {
		icon  string
		label string
		fn    func()
	}{
		{"close", "Close Window", func() {
			if refs.Hyprland != nil && w.capturedWin != "" {
				go refs.Hyprland.CloseWindow(w.capturedWin)
			}
		}},
		{"fullscreen", "Toggle Fullscreen", func() {
			if refs.Hyprland != nil && w.capturedWin != "" {
				go refs.Hyprland.ToggleFullscreenWindow(w.capturedWin)
			}
		}},
		{"picture_in_picture_alt", "Toggle Floating", func() {
			if refs.Hyprland != nil && w.capturedWin != "" {
				go refs.Hyprland.ToggleFloatingWindow(w.capturedWin)
			}
		}},
		{"view_column", "Change Split", func() {
			if refs.Hyprland != nil && w.capturedWin != "" {
				go refs.Hyprland.ToggleSplitWindow(w.capturedWin)
			}
		}},
	}

	for _, a := range actions {
		row := actionRow(a.icon, a.label, a.fn)
		box.Append(row)
	}

	// Divider.
	box.Append(gtkutil.M3Divider())

	// Move to workspace header.
	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	header.AddCSSClass("conn-section-header")
	header.SetMarginStart(16)
	header.SetMarginEnd(16)

	label := gtk.NewLabel("Move to workspace")
	label.AddCSSClass("conn-section-title")
	label.SetHExpand(true)
	header.Append(label)
	box.Append(header)

	// Workspace rows - use captured window address.
	for i := range maxWorkspaces {
		wsID := i + 1
		lbl := fmt.Sprintf("Workspace %d", wsID)
		id := wsID
		row := actionRow("space_dashboard", lbl, func() {
			if refs.Hyprland != nil && w.capturedWin != "" {
				go refs.Hyprland.MoveWindowToWorkspace(w.capturedWin, id)
			}
		})
		if wsID == w.activeWS {
			row.AddCSSClass("conn-row-connected")
		}
		w.wsRows = append(w.wsRows, row)
		box.Append(row)
	}

	return box
}

func actionRow(icon, label string, onActivate func()) *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.AddCSSClass("conn-row")
	row.SetCursorFromName("pointer")

	iconLabel := gtkutil.MaterialIcon(icon)
	iconLabel.AddCSSClass("conn-row-icon")

	textLabel := gtk.NewLabel(label)
	textLabel.AddCSSClass("conn-row-label")

	row.Append(iconLabel)
	row.Append(textLabel)

	gtkutil.ClaimedClick(&row.Widget, onActivate)

	return row
}
