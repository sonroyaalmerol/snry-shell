// Package polkit provides a GUI PolicyKit1 authentication agent.
//
// It registers on the system bus as org.freedesktop.PolicyKit1.AuthenticationAgent,
// shows a GTK password dialog when authentication is requested, and responds via
// the Authority's AuthenticationAgentResponse2 method.
package polkit

import (
	"fmt"
	"os"
	"sync"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/godbus/dbus/v5"
)

const (
	authAgentPath  = "/org/freedesktop/PolicyKit1/AuthenticationAgent"
	authAgentIface = "org.freedesktop.PolicyKit1.AuthenticationAgent"
	authorityBus   = "org.freedesktop.PolicyKit1"
	authorityPath  = "/org/freedesktop/PolicyKit1/Authority"
	authorityIface = "org.freedesktop.PolicyKit1.Authority"
)

// Identity represents a polkit identity: (kind, details).
type Identity struct {
	Kind    string
	Details map[string]dbus.Variant
}

// Agent implements the polkit authentication agent interface.
type Agent struct {
	conn      *dbus.Conn
	pending   map[string]*authRequest
	pendingMu sync.Mutex
}

type authRequest struct {
	cookie    string
	identity  Identity
	actionID  string
	message   string
	iconName  string
	cancelled bool
}

// New creates a new polkit agent.
func New(conn *dbus.Conn) *Agent {
	return &Agent{
		conn:    conn,
		pending: make(map[string]*authRequest),
	}
}

// Register exports the agent on the system bus and registers with the Authority.
func (a *Agent) Register() error {
	if a.conn == nil {
		return fmt.Errorf("polkit: no system bus connection")
	}

	if err := a.conn.Export(a, authAgentPath, authAgentIface); err != nil {
		return fmt.Errorf("polkit export: %w", err)
	}

	// Get session ID from environment or /proc.
	sessionID := os.Getenv("XDG_SESSION_ID")
	if sessionID == "" {
		if data, err := os.ReadFile("/proc/self/sessionid"); err == nil {
			sessionID = string(data)
		} else {
			sessionID = "c1"
		}
	}

	// Register with the Authority.
	auth := a.conn.Object(authorityBus, authorityPath)
	subject := []interface{}{"unix-session", map[string]dbus.Variant{
		"session-id": dbus.MakeVariant(sessionID),
	}}
	err := auth.Call(authorityIface+".RegisterAuthenticationAgent", 0,
		subject, "", authAgentPath,
	).Err
	if err != nil {
		return fmt.Errorf("polkit register: %w", err)
	}

	return nil
}

// BeginAuthentication is called by polkitd when an application needs authentication.
func (a *Agent) BeginAuthentication(
	actionID string,
	message string,
	iconName string,
	details map[string]string,
	cookie string,
	identities []Identity,
) *dbus.Error {
	if len(identities) == 0 {
		return dbus.MakeFailedError(fmt.Errorf("polkit: no identities provided"))
	}

	// Pick the first unix-user identity.
	identity := identities[0]
	for _, id := range identities {
		if id.Kind == "unix-user" {
			identity = id
			break
		}
	}

	req := &authRequest{
		cookie:    cookie,
		identity:  identity,
		actionID:  actionID,
		message:   message,
		iconName:  iconName,
	}

	a.pendingMu.Lock()
	a.pending[cookie] = req
	a.pendingMu.Unlock()

	// Show dialog on the GTK main thread.
	glib.IdleAdd(func() {
		a.showDialog(req)
	})

	return nil
}

// CancelAuthentication is called by polkitd when the auth request is cancelled.
func (a *Agent) CancelAuthentication(cookie string) *dbus.Error {
	a.pendingMu.Lock()
	if req, ok := a.pending[cookie]; ok {
		req.cancelled = true
		delete(a.pending, cookie)
	}
	a.pendingMu.Unlock()
	return nil
}

// showDialog shows a modal GTK dialog asking for the user's password.
func (a *Agent) showDialog(req *authRequest) {
	win := gtk.NewWindow()
	win.SetTitle("Authentication Required")
	win.SetModal(true)
	win.SetDecorated(false)
	win.SetName("snry-polkit-dialog")
	win.SetDefaultSize(380, 180)

	root := gtk.NewBox(gtk.OrientationVertical, 16)
	root.AddCSSClass("polkit-dialog")

	// Header row: icon + message.
	header := gtk.NewBox(gtk.OrientationHorizontal, 12)

	icon := gtk.NewLabel("lock")
	icon.AddCSSClass("material-icon-lg")
	icon.AddCSSClass("polkit-icon")

	msgLabel := gtk.NewLabel(req.message)
	msgLabel.AddCSSClass("polkit-message")
	msgLabel.SetWrap(true)
	msgLabel.SetXAlign(0)
	msgLabel.SetHExpand(true)

	header.Append(icon)
	header.Append(msgLabel)
	root.Append(header)

	// Action label.
	actionLabel := gtk.NewLabel(fmt.Sprintf("Action: %s", req.actionID))
	actionLabel.AddCSSClass("polkit-action")
	actionLabel.SetWrap(true)
	actionLabel.SetXAlign(0)
	root.Append(actionLabel)

	// Password entry.
	entry := gtk.NewEntry()
	entry.AddCSSClass("polkit-password-entry")
	entry.SetPlaceholderText("Password")
	entry.SetVisibility(false)
	entry.SetHExpand(true)
	root.Append(entry)

	// Buttons.
	btnBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
	btnBox.SetHAlign(gtk.AlignEnd)

	cancelBtn := gtk.NewButtonWithLabel("Cancel")
	cancelBtn.AddCSSClass("polkit-cancel-btn")

	authBtn := gtk.NewButtonWithLabel("Authenticate")
	authBtn.AddCSSClass("polkit-auth-btn")

	btnBox.Append(cancelBtn)
	btnBox.Append(authBtn)
	root.Append(btnBox)

	win.SetChild(root)

	cancelBtn.ConnectClicked(func() {
		a.pendingMu.Lock()
		delete(a.pending, req.cookie)
		a.pendingMu.Unlock()
		win.Close()
	})

	authBtn.ConnectClicked(func() {
		password := entry.Text()
		win.Close()
		go a.respondAuth(req.cookie, req.identity, password)
	})

	entry.ConnectActivate(func() {
		password := entry.Text()
		win.Close()
		go a.respondAuth(req.cookie, req.identity, password)
	})

	win.Present()
	entry.GrabFocus()
}

// respondAuth calls the Authority's AuthenticationAgentResponse2 method.
func (a *Agent) respondAuth(cookie string, identity Identity, password string) {
	a.pendingMu.Lock()
	req, ok := a.pending[cookie]
	a.pendingMu.Unlock()

	if !ok || req.cancelled {
		return
	}

	// Build identity struct for the response: (sa{sv})
	ident := []interface{}{identity.Kind, identity.Details}
	uid := uint32(os.Getuid())

	auth := a.conn.Object(authorityBus, authorityPath)
	err := auth.Call(authorityIface+".AuthenticationAgentResponse2", 0,
		uid, cookie, ident,
	).Err

	if err != nil {
		fmt.Fprintf(os.Stderr, "polkit: authentication response failed: %v\n", err)
	}

	a.pendingMu.Lock()
	delete(a.pending, cookie)
	a.pendingMu.Unlock()

	// Clear password from memory.
	_ = password
}
