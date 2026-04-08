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
	win     *gtk.ApplicationWindow
	bus     *bus.Bus
	refs    *servicerefs.ServiceRefs
	trigger gtk.Widgetter
	monitor *gdk.Monitor
	root    *gtk.Box
	wsRows  []*gtk.Box
	activeWS int

	capturedWin      string
	capturedFloating bool
	isOpen           bool

	widthScale        *gtk.Scale
	heightScale       *gtk.Scale
	resizeDebounce    glib.SourceHandle
	settingInitialVal bool
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

	scroll.SetChild(w.buildContent())
	panel.Append(scroll)

	root.Append(panel)

	// Reset state whenever the popup is hidden by any means (scrim click, escape, etc.)
	win.ConnectHide(func() {
		w.isOpen = false
		w.capturedWin = ""
		w.capturedFloating = false
	})

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

	return w
}

func (w *WindowMgmt) hidePopup() {
	w.win.SetVisible(false)
	// ConnectHide will reset isOpen / capturedWin / capturedFloating.
}

func (w *WindowMgmt) Toggle() {
	if w.win.Visible() {
		w.hidePopup()
	} else {
		// Capture the current window before opening.
		if w.refs.Hyprland != nil {
			if win, err := w.refs.Hyprland.ActiveWindow(); err == nil && win.Address != "" {
				w.capturedWin = win.Address
				w.capturedFloating = win.Floating

				// Update sliders with current window size without triggering a resize.
				if win.Size[0] > 0 && win.Size[1] > 0 {
					w.settingInitialVal = true
					if w.widthScale != nil {
						w.widthScale.SetValue(float64(win.Size[0]))
					}
					if w.heightScale != nil {
						w.heightScale.SetValue(float64(win.Size[1]))
					}
					w.settingInitialVal = false
				}
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

func (w *WindowMgmt) scheduleResize() {
	if w.resizeDebounce != 0 {
		glib.SourceRemove(w.resizeDebounce)
		w.resizeDebounce = 0
	}
	w.resizeDebounce = glib.TimeoutAdd(80, func() bool {
		w.resizeDebounce = 0
		if w.refs.Hyprland == nil || w.capturedWin == "" {
			return false
		}
		wVal := int(w.widthScale.Value())
		hVal := int(w.heightScale.Value())
		addr := w.capturedWin
		wasFloating := w.capturedFloating
		go func() {
			if !wasFloating {
				// Float the window first so resize takes effect.
				w.refs.Hyprland.ToggleFloatingWindow(addr)
				glib.IdleAdd(func() { w.capturedFloating = true })
			}
			w.refs.Hyprland.ResizeWindow(addr, wVal, hVal)
		}()
		return false
	})
}

func (w *WindowMgmt) buildContent() gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("windowmgmt-widget")

	// ── Actions ────────────────────────────────────────────────────────────
	actionsHeader := gtk.NewLabel("Actions")
	actionsHeader.AddCSSClass("windowmgmt-section-header")
	actionsHeader.SetHAlign(gtk.AlignStart)
	box.Append(actionsHeader)

	actions := []struct {
		icon  string
		label string
		fn    func()
	}{
		{"close", "Close", func() {
			if w.refs.Hyprland != nil && w.capturedWin != "" {
				addr := w.capturedWin
				w.hidePopup()
				go w.refs.Hyprland.CloseWindow(addr)
			}
		}},
		{"fullscreen", "Fullscreen", func() {
			if w.refs.Hyprland != nil && w.capturedWin != "" {
				addr := w.capturedWin
				w.hidePopup()
				go w.refs.Hyprland.ToggleFullscreenWindow(addr)
			}
		}},
		{"picture_in_picture_alt", "Float", func() {
			if w.refs.Hyprland != nil && w.capturedWin != "" {
				addr := w.capturedWin
				w.hidePopup()
				go w.refs.Hyprland.ToggleFloatingWindow(addr)
			}
		}},
		{"view_column", "Split", func() {
			if w.refs.Hyprland != nil && w.capturedWin != "" {
				addr := w.capturedWin
				w.hidePopup()
				go w.refs.Hyprland.ToggleSplitWindow(addr)
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

	// ── Resize ─────────────────────────────────────────────────────────────
	box.Append(gtkutil.M3Divider())

	resizeHeader := gtk.NewLabel("Resize")
	resizeHeader.AddCSSClass("windowmgmt-section-header")
	resizeHeader.SetHAlign(gtk.AlignStart)
	box.Append(resizeHeader)

	resizeBox := gtk.NewBox(gtk.OrientationVertical, 8)
	resizeBox.AddCSSClass("windowmgmt-resize-box")

	// Width slider
	wRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	wRow.AddCSSClass("windowmgmt-resize-row")
	wLbl := gtk.NewLabel("W")
	wLbl.AddCSSClass("windowmgmt-resize-label")
	wLbl.SetSizeRequest(16, -1)
	w.widthScale = gtkutil.M3Slider(200, 3840, 10)
	w.widthScale.SetValue(800)
	w.widthScale.ConnectValueChanged(func() {
		if !w.settingInitialVal {
			w.scheduleResize()
		}
	})
	wRow.Append(wLbl)
	wRow.Append(w.widthScale)

	// Height slider
	hRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	hRow.AddCSSClass("windowmgmt-resize-row")
	hLbl := gtk.NewLabel("H")
	hLbl.AddCSSClass("windowmgmt-resize-label")
	hLbl.SetSizeRequest(16, -1)
	w.heightScale = gtkutil.M3Slider(150, 2160, 10)
	w.heightScale.SetValue(600)
	w.heightScale.ConnectValueChanged(func() {
		if !w.settingInitialVal {
			w.scheduleResize()
		}
	})
	hRow.Append(hLbl)
	hRow.Append(w.heightScale)

	resizeBox.Append(wRow)
	resizeBox.Append(hRow)
	box.Append(resizeBox)

	// ── Move to Workspace ──────────────────────────────────────────────────
	box.Append(gtkutil.M3Divider())

	wsHeader := gtk.NewLabel("Move to Workspace")
	wsHeader.AddCSSClass("windowmgmt-section-header")
	wsHeader.SetHAlign(gtk.AlignStart)
	box.Append(wsHeader)

	wsGrid := gtk.NewGrid()
	wsGrid.AddCSSClass("windowmgmt-ws-grid")
	wsGrid.SetColumnSpacing(6)
	wsGrid.SetRowSpacing(6)
	wsGrid.SetColumnHomogeneous(true)

	for i := range maxWorkspaces {
		wsID := i + 1
		id := wsID
		btn := workspaceButton(wsID, func() {
			if w.refs.Hyprland != nil && w.capturedWin != "" {
				addr := w.capturedWin
				w.hidePopup()
				go w.refs.Hyprland.MoveWindowToWorkspace(addr, id)
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
