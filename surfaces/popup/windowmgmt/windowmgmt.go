package windowmgmt

import (
	"strconv"

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

	// Header
	header := gtk.NewLabel("Window")
	header.AddCSSClass("popup-header")
	header.SetHAlign(gtk.AlignStart)
	panel.Append(header)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.AddCSSClass("popup-scroll")
	scroll.SetMaxContentHeight(surfaceutil.PopupMaxHeight(w.monitor, layershell.BarHeight()))
	scroll.SetPropagateNaturalHeight(true)

	scroll.SetChild(buildContent(b, refs, w))
	panel.Append(scroll)

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
			row.AddCSSClass("windowmgmt-ws-active")
		} else {
			row.RemoveCSSClass("windowmgmt-ws-active")
		}
	}
}

func buildContent(b *bus.Bus, refs *servicerefs.ServiceRefs, w *WindowMgmt) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("windowmgmt-widget")

	// Actions section header
	actionsHeader := gtk.NewLabel("Actions")
	actionsHeader.AddCSSClass("windowmgmt-section-header")
	actionsHeader.SetHAlign(gtk.AlignStart)
	box.Append(actionsHeader)

	// Action rows - use captured window address.
	actions := []struct {
		icon  string
		label string
		fn    func()
	}{
		{"close", "Close", func() {
			if refs.Hyprland != nil && w.capturedWin != "" {
				go refs.Hyprland.CloseWindow(w.capturedWin)
			}
		}},
		{"fullscreen", "Fullscreen", func() {
			if refs.Hyprland != nil && w.capturedWin != "" {
				go refs.Hyprland.ToggleFullscreenWindow(w.capturedWin)
			}
		}},
		{"picture_in_picture_alt", "Float", func() {
			if refs.Hyprland != nil && w.capturedWin != "" {
				go refs.Hyprland.ToggleFloatingWindow(w.capturedWin)
			}
		}},
		{"view_column", "Split", func() {
			if refs.Hyprland != nil && w.capturedWin != "" {
				go refs.Hyprland.ToggleSplitWindow(w.capturedWin)
			}
		}},
	}

	actionsGrid := gtk.NewGrid()
	actionsGrid.AddCSSClass("windowmgmt-actions-grid")
	actionsGrid.SetColumnSpacing(8)
	actionsGrid.SetRowSpacing(8)
	actionsGrid.SetColumnHomogeneous(true)

	for i, a := range actions {
		btn := actionButton(a.icon, a.label, a.fn)
		actionsGrid.Attach(btn, i%2, i/2, 1, 1)
	}
	box.Append(actionsGrid)

	// Divider.
	box.Append(gtkutil.M3Divider())

	// Workspaces section header
	wsHeader := gtk.NewLabel("Move to Workspace")
	wsHeader.AddCSSClass("windowmgmt-section-header")
	wsHeader.SetHAlign(gtk.AlignStart)
	box.Append(wsHeader)

	// Workspace grid - 2 columns for compact layout
	wsGrid := gtk.NewGrid()
	wsGrid.AddCSSClass("windowmgmt-ws-grid")
	wsGrid.SetColumnSpacing(6)
	wsGrid.SetRowSpacing(6)
	wsGrid.SetColumnHomogeneous(true)

	// Workspace buttons - use captured window address.
	for i := range maxWorkspaces {
		wsID := i + 1
		id := wsID
		btn := workspaceButton(wsID, func() {
			if refs.Hyprland != nil && w.capturedWin != "" {
				go refs.Hyprland.MoveWindowToWorkspace(w.capturedWin, id)
			}
		})
		if wsID == w.activeWS {
			btn.AddCSSClass("windowmgmt-ws-active")
		}
		w.wsRows = append(w.wsRows, btn)
		wsGrid.Attach(btn, i%2, i/2, 1, 1)
	}
	box.Append(wsGrid)

	return box
}

func actionButton(icon, label string, onActivate func()) *gtk.Button {
	btn := gtk.NewButton()
	btn.AddCSSClass("windowmgmt-action-btn")
	btn.SetCursorFromName("pointer")
	btn.SetHExpand(true)

	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.SetHAlign(gtk.AlignCenter)
	box.SetVAlign(gtk.AlignCenter)

	iconLabel := gtkutil.MaterialIcon(icon)
	iconLabel.AddCSSClass("windowmgmt-action-icon")

	textLabel := gtk.NewLabel(label)
	textLabel.AddCSSClass("windowmgmt-action-label")

	box.Append(iconLabel)
	box.Append(textLabel)
	btn.SetChild(box)

	btn.ConnectClicked(onActivate)

	return btn
}

func workspaceButton(wsID int, onActivate func()) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("windowmgmt-ws-btn")
	box.SetCursorFromName("pointer")
	box.SetHExpand(true)

	label := gtk.NewLabel(strconv.Itoa(wsID))
	label.AddCSSClass("windowmgmt-ws-label")
	label.SetHExpand(true)
	label.SetHAlign(gtk.AlignCenter)

	box.Append(label)

	gtkutil.ClaimedClick(&box.Widget, onActivate)

	return box
}
