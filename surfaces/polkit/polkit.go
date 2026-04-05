// Package polkit provides a GUI PolicyKit1 authentication agent.
//
// It registers on the system bus as org.freedesktop.PolicyKit1.AuthenticationAgent,
// shows a GTK password dialog when authentication is requested, and authenticates
// via the polkit-agent-helper-1 setuid binary for proper PAM verification.
package polkit

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
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

	helperPath = "/usr/lib/polkit-1/polkit-agent-helper-1"
)

// Identity represents a polkit identity: (kind, details).
type Identity struct {
	Kind    string
	Details map[string]dbus.Variant
}

// Agent implements the polkit authentication agent interface.
type Agent struct {
	conn      *dbus.Conn
	pending   map[string]*helperSession
	pendingMu sync.Mutex
}

// helperSession manages a single authentication conversation with
// polkit-agent-helper-1 via stdin/stdout pipes.
type helperSession struct {
	agent   *Agent
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	cookie  string
	req     *authRequest
	win     *gtk.Window
	entry   *gtk.Entry
	infoLbl *gtk.Label
}

type authRequest struct {
	identity  Identity
	actionID  string
	message   string
	iconName  string
}

// New creates a new polkit agent.
func New(conn *dbus.Conn) *Agent {
	return &Agent{
		conn:    conn,
		pending: make(map[string]*helperSession),
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
		identity: identity,
		actionID: actionID,
		message:  message,
		iconName: iconName,
	}

	a.pendingMu.Lock()
	a.pending[cookie] = &helperSession{
		agent: a,
		cookie: cookie,
		req:   req,
	}
	a.pendingMu.Unlock()

	// Show dialog on the GTK main thread.
	glib.IdleAdd(func() {
		a.startSession(cookie, req)
	})

	return nil
}

// CancelAuthentication is called by polkitd when the auth request is cancelled.
func (a *Agent) CancelAuthentication(cookie string) *dbus.Error {
	a.pendingMu.Lock()
	if sess, ok := a.pending[cookie]; ok {
		if sess.cmd != nil && sess.cmd.Process != nil {
			sess.cmd.Process.Kill()
		}
		if sess.win != nil {
			glib.IdleAdd(func() { sess.win.Close() })
		}
		delete(a.pending, cookie)
	}
	a.pendingMu.Unlock()
	return nil
}

// startSession spawns the helper and shows the dialog.
func (a *Agent) startSession(cookie string, req *authRequest) {
	// Extract username from identity UID.
	username := "root"
	if uidVar, ok := req.identity.Details["uid"]; ok {
		if uid, ok := uidVar.Value().(uint32); ok {
			if u, err := user.LookupId(strconv.Itoa(int(uid))); err == nil {
				username = u.Username
			}
		}
	}

	// Check helper exists.
	if _, err := os.Stat(helperPath); err != nil {
		fmt.Fprintf(os.Stderr, "polkit: %s not found, skipping PAM auth\n", helperPath)
		a.cancelSession(cookie)
		return
	}

	// Spawn the helper.
	cmd := exec.Command(helperPath, username)
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "polkit: failed to create stdin pipe: %v\n", err)
		a.cancelSession(cookie)
		return
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "polkit: failed to create stdout pipe: %v\n", err)
		a.cancelSession(cookie)
		return
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "polkit: failed to spawn helper: %v\n", err)
		a.cancelSession(cookie)
		return
	}

	a.pendingMu.Lock()
	sess, ok := a.pending[cookie]
	if !ok {
		cmd.Process.Kill()
		a.pendingMu.Unlock()
		return
	}
	sess.cmd = cmd
	sess.stdin = stdinPipe
	sess.scanner = bufio.NewScanner(stdoutPipe)
	a.pendingMu.Unlock()

	// Send cookie to helper.
	fmt.Fprintf(stdinPipe, "%s\n", cookie)

	// Show the dialog.
	sess.win, sess.entry, sess.infoLbl = a.buildDialog(cookie, req)

	// Read helper responses in background.
	go sess.readLoop()
}

// buildDialog creates the authentication dialog and returns window, entry, info label.
func (a *Agent) buildDialog(cookie string, req *authRequest) (*gtk.Window, *gtk.Entry, *gtk.Label) {
	win := gtk.NewWindow()
	win.SetTitle("Authentication Required")
	win.SetModal(true)
	win.SetDecorated(false)
	win.SetName("snry-polkit-dialog")
	win.SetDefaultSize(380, 200)

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

	// Info/error label (shown when helper sends messages).
	infoLbl := gtk.NewLabel("")
	infoLbl.AddCSSClass("polkit-info")
	infoLbl.SetWrap(true)
	infoLbl.SetXAlign(0)
	root.Append(infoLbl)

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
	cancelBtn.SetCursorFromName("pointer")
	cancelBtn.AddCSSClass("polkit-cancel-btn")

	authBtn := gtk.NewButtonWithLabel("Authenticate")
	authBtn.SetCursorFromName("pointer")
	authBtn.AddCSSClass("polkit-auth-btn")

	btnBox.Append(cancelBtn)
	btnBox.Append(authBtn)
	root.Append(btnBox)
	win.SetChild(root)

	// Cancel: kill helper and close dialog.
	cancelBtn.ConnectClicked(func() {
		a.cancelSession(cookie)
		win.Close()
	})

	// Authenticate: send password to helper.
	sendPassword := func() {
		password := entry.Text()
		entry.SetText("")
		a.sendResponse(cookie, password)
	}
	authBtn.ConnectClicked(sendPassword)
	entry.ConnectActivate(sendPassword)

	win.Present()
	entry.GrabFocus()

	return win, entry, infoLbl
}

// readLoop reads lines from the helper's stdout and dispatches them.
func (s *helperSession) readLoop() {
	for s.scanner.Scan() {
		line := gStrUnescape(s.scanner.Text())

		switch {
		case strings.HasPrefix(line, "PAM_PROMPT_ECHO_OFF "):
			strings.TrimPrefix(line, "PAM_PROMPT_ECHO_OFF ")
			glib.IdleAdd(func() {
				if s.infoLbl != nil {
					s.infoLbl.SetText("")
				}
				if s.entry != nil {
					s.entry.GrabFocus()
				}
			})

		case strings.HasPrefix(line, "PAM_PROMPT_ECHO_ON "):
			prompt := strings.TrimPrefix(line, "PAM_PROMPT_ECHO_ON ")
			glib.IdleAdd(func() {
				if s.entry != nil {
					s.entry.SetVisibility(true)
					s.entry.GrabFocus()
				}
			})
			_ = prompt

		case strings.HasPrefix(line, "PAM_ERROR_MSG "):
			msg := strings.TrimPrefix(line, "PAM_ERROR_MSG ")
			glib.IdleAdd(func() {
				if s.infoLbl != nil {
					s.infoLbl.SetText(msg)
				}
			})

		case strings.HasPrefix(line, "PAM_TEXT_INFO "):
			msg := strings.TrimPrefix(line, "PAM_TEXT_INFO ")
			glib.IdleAdd(func() {
				if s.infoLbl != nil {
					s.infoLbl.SetText(msg)
				}
			})

		case line == "SUCCESS":
			glib.IdleAdd(func() {
				if s.win != nil {
					s.win.Close()
				}
			})
			s.cleanup()
			return

		case line == "FAILURE":
			glib.IdleAdd(func() {
				if s.infoLbl != nil {
					s.infoLbl.SetText("Authentication failed")
				}
				if s.entry != nil {
					s.entry.SetText("")
					s.entry.GrabFocus()
				}
			})
			// Don't return — wait for another prompt or kill.

		default:
			// Unknown line, ignore.
		}
	}
	// Scanner ended (EOF / error) — clean up.
	s.cleanup()
}

// sendResponse writes the user's password to the helper's stdin.
func (a *Agent) sendResponse(cookie string, response string) {
	a.pendingMu.Lock()
	sess, ok := a.pending[cookie]
	a.pendingMu.Unlock()

	if !ok || sess.stdin == nil {
		return
	}

	fmt.Fprintf(sess.stdin, "%s\n", response)
}

// cancelSession kills the helper, closes the dialog, and removes from pending.
func (a *Agent) cancelSession(cookie string) {
	a.pendingMu.Lock()
	sess, ok := a.pending[cookie]
	delete(a.pending, cookie)
	a.pendingMu.Unlock()

	if !ok {
		return
	}
	if sess.cmd != nil && sess.cmd.Process != nil {
		sess.cmd.Process.Kill()
	}
}

// cleanup removes the session from pending.
func (s *helperSession) cleanup() {
	s.agent.pendingMu.Lock()
	delete(s.agent.pending, s.cookie)
	s.agent.pendingMu.Unlock()

	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Wait()
	}
}

// gStrUnescape reverses GLib's g_strescape() C-style escaping.
func gStrUnescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte(s[i+1])
			}
			i++
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
