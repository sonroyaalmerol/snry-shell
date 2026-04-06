// Package inputmode manages the global input mode (auto/tablet/desktop)
// and publishes an effective tablet-mode boolean for the rest of the shell.
//
// In "auto" mode the service combines logind's TabletMode property with a
// physical-keyboard-activity heuristic to decide whether the OSK should
// auto-trigger.  On devices with a proper hardware switch (e.g. ThinkPad,
// Surface Pro 8+) logind is authoritative.  On devices where logind reports
// "indeterminate" (e.g. Surface Pro 7+) the service falls back to: if no
// physical keyboard activity has been detected recently, assume tablet mode.
package inputmode

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/dbusutil"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
)

const (
	logindDest  = "org.freedesktop.login1"
	logindIface = "org.freedesktop.login1.Session"
	logindProp  = "TabletMode"

	kbInactivityTimeout = 30 * time.Second
)

// virtualKeyboardNames matches device names that are NOT real physical keyboards.
var virtualKeyboardNames = []string{
	"snry-osk-virtual",
	"ydotoold",
	"virtual",
	"power-button",
	"sleep-button",
	"lid-switch",
}

// Service manages input mode and publishes effective tablet-mode state.
type Service struct {
	bus  *bus.Bus
	conn dbusutil.DBusConn
	mu   sync.Mutex

	mode       string // "auto", "tablet", "desktop"
	logindMode string // "enabled", "disabled", "indeterminate"
	kbActive   bool
	kbTimer    *time.Timer
	hasTouch   bool
	session    string
}

// New creates the service.  hasTouch should reflect whether a touchscreen
// is available (used as a fallback in auto mode).
func New(b *bus.Bus, conn *dbus.Conn, cfg settings.Config, hasTouch bool) *Service {
	s := &Service{
		bus:      b,
		conn:     dbusutil.NewRealConn(conn),
		mode:     cfg.InputMode,
		hasTouch: hasTouch,
	}
	if s.mode == "" {
		s.mode = "auto"
	}
	return s
}

// Run starts logind monitoring, keyboard activity monitoring and the
// system-controls listener.  Blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	go s.monitorLogind(ctx)
	go s.monitorKeyboard(ctx)

	s.bus.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		action, _ := e.Data.(string)
		if strings.HasPrefix(action, "set-input-mode:") {
			s.SetMode(strings.TrimPrefix(action, "set-input-mode:"))
		}
	})

	// Listen for external settings changes (e.g. from control panel)
	s.bus.Subscribe(bus.TopicSettingsChanged, func(e bus.Event) {
		if cfg, ok := e.Data.(settings.Config); ok {
			s.mu.Lock()
			if s.mode != cfg.InputMode {
				s.mode = cfg.InputMode
				if s.mode == "" {
					s.mode = "auto"
				}
				s.mu.Unlock()
				s.publish()
			} else {
				s.mu.Unlock()
			}
		}
	})

	// Detect initial touch state.
	s.hasTouch = detectTouchDevice()

	// Publish initial state immediately so late subscribers get the saved mode.
	s.publish()

	// Re-publish after keyboard monitor has had time to detect activity.
	time.AfterFunc(500*time.Millisecond, func() { s.publish() })

	<-ctx.Done()
	return ctx.Err()
}

// SetMode changes the input mode, persists it, and republishes.
func (s *Service) SetMode(mode string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch mode {
	case "auto", "tablet", "desktop":
		s.mode = mode
	default:
		return
	}

	if err := s.persist(); err != nil {
		log.Printf("[INPUTMODE] persist: %v", err)
	}
	s.publish()
}

// persist saves the current mode to the settings file.
func (s *Service) persist() error {
	cfg, err := settings.Load()
	if err != nil {
		return err
	}
	cfg.InputMode = s.mode
	return settings.Save(cfg)
}

// publish calculates the effective tablet-mode boolean and publishes to the bus.
func (s *Service) publish() {
	tablet := false
	switch s.mode {
	case "tablet":
		tablet = true
	case "desktop":
		tablet = false
	case "auto":
		switch s.logindMode {
		case "enabled":
			tablet = true
		case "disabled":
			tablet = false
		default: // "indeterminate"
			tablet = !s.kbActive && s.hasTouch
		}
	}
	log.Printf("[INPUTMODE] mode=%s logind=%s kb=%v touch=%v → tablet=%v",
		s.mode, s.logindMode, s.kbActive, s.hasTouch, tablet)
	s.bus.Publish(bus.TopicTabletMode, tablet)
	s.bus.Publish(bus.TopicInputMode, s.mode)
}

// ── logind monitor ──────────────────────────────────────────────────────────

func (s *Service) monitorLogind(ctx context.Context) {
	if s.conn == nil {
		log.Printf("[INPUTMODE] no system bus, skipping logind monitor")
		return
	}
	session, err := s.resolveSession()
	if err != nil {
		log.Printf("[INPUTMODE] cannot resolve session: %v", err)
		return
	}
	s.session = session

	ch := make(chan *dbus.Signal, 16)
	s.conn.Signal(ch)

	if err := s.conn.AddMatchSignal(dbus.WithMatchObjectPath(dbus.ObjectPath(session))); err != nil {
		log.Printf("[INPUTMODE] AddMatchSignal: %v", err)
		return
	}

	s.queryLogind()

	for {
		select {
		case <-ctx.Done():
			return
		case sig, ok := <-ch:
			if !ok {
				return
			}
			if sig.Path != dbus.ObjectPath(session) {
				continue
			}
			s.queryLogind()
		}
	}
}

func (s *Service) resolveSession() (string, error) {
	if id := os.Getenv("XDG_SESSION_ID"); id != "" {
		path := "/org/freedesktop/login1/session_" + id
		if s.conn.Object(logindDest, dbus.ObjectPath(path)) != nil {
			return path, nil
		}
	}
	mgr := s.conn.Object(logindDest, "/org/freedesktop/login1")
	var sessionPath dbus.ObjectPath
	if err := mgr.Call("org.freedesktop.login1.Manager.GetSessionByPID", 0, uint32(os.Getpid())).Store(&sessionPath); err != nil {
		return "", err
	}
	return string(sessionPath), nil
}

func (s *Service) queryLogind() {
	if s.conn == nil || s.session == "" {
		return
	}
	obj := s.conn.Object(logindDest, dbus.ObjectPath(s.session))
	v, err := obj.GetProperty(logindIface + "." + logindProp)
	if err != nil {
		s.mu.Lock()
		s.logindMode = "indeterminate"
		s.mu.Unlock()
		return
	}
	mode, ok := v.Value().(string)
	if !ok {
		s.mu.Lock()
		s.logindMode = "indeterminate"
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	s.logindMode = mode
	s.mu.Unlock()
	s.publish()
}

// ── keyboard activity monitor ──────────────────────────────────────────────────

func (s *Service) monitorKeyboard(ctx context.Context) {
	devices := findPhysicalKeyboardDevices()
	if len(devices) == 0 {
		log.Printf("[INPUTMODE] no physical keyboard devices found")
		return
	}
	log.Printf("[INPUTMODE] monitoring %d keyboard device(s): %v", len(devices), devices)

	for _, dev := range devices {
		go s.readKeyboard(ctx, dev)
	}
}

func (s *Service) readKeyboard(ctx context.Context, devPath string) {
	f, err := os.Open(devPath)
	if err != nil {
		return
	}
	defer f.Close()

	buf := make([]byte, 24)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := f.Read(buf)
			if err != nil {
				return
			}
			if n != 24 {
				continue
			}
			typ := binary.LittleEndian.Uint16(buf[16:18])
			val := binary.LittleEndian.Uint32(buf[20:24])
			if typ == 1 && val == 1 { // EV_KEY pressed
				s.onKeyboardActivity()
			}
		}
	}
}

func (s *Service) onKeyboardActivity() {
	s.mu.Lock()
	s.kbActive = true
	s.mu.Unlock()

	// Reset inactivity timer.
	if s.kbTimer != nil {
		s.kbTimer.Stop()
	}
	s.kbTimer = time.AfterFunc(kbInactivityTimeout, func() {
		s.mu.Lock()
		s.kbActive = false
		s.mu.Unlock()
		s.publish()
	})
	s.publish()
}

// findPhysicalKeyboardDevices parses /proc/bus/input/devices and returns
// event device paths for physical keyboards (not virtual ones).
func findPhysicalKeyboardDevices() []string {
	data, err := os.ReadFile("/proc/bus/input/devices")
	if err != nil {
		return nil
	}

	var devices []string
	var name string
	inHandlers := false

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// End of device block.
			if inHandlers {
				devices = appendEventDevice(devices, line, name)
			}
			name = ""
			inHandlers = false
			continue
		}
		if strings.HasPrefix(line, "N: Name=") {
			name = strings.TrimPrefix(line, "N: Name=\"")
			name = strings.TrimSuffix(name, "\"")
		}
		if strings.HasPrefix(line, "H: Handlers=") {
			if strings.Contains(line, "kbd") && !isVirtualKeyboard(name) {
				inHandlers = true
			}
		}
	}
	return devices
}

func appendEventDevice(out []string, handlerLine, name string) []string {
	for _, field := range strings.Split(handlerLine, " ") {
		if strings.HasPrefix(field, "event") {
			return append(out, "/dev/input/"+field)
		}
	}
	return out
}

func isVirtualKeyboard(name string) bool {
	lower := strings.ToLower(name)
	for _, pat := range virtualKeyboardNames {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

// detectTouchDevice checks for touch device availability.
func detectTouchDevice() bool {
	out, err := os.ReadFile("/proc/bus/input/devices")
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "touch")
}
