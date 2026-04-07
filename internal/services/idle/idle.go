// Package idle monitors user inactivity and triggers screen locking and
// optional system suspend. It replaces the functionality of hypridle.
//
// Activity is detected by reading raw input events from /dev/input/event*
// (requires the user to be in the 'input' group, which is standard on
// Hyprland setups). Any EV_KEY, EV_REL (mouse), or EV_ABS (touch) event
// resets the idle timer.
//
// Logind integration: PrepareForSleep locks the screen before suspend;
// the session Lock/Unlock D-Bus signals make loginctl lock-session
// interoperable with the shell lockscreen.
package idle

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const (
	evKey uint16 = 1 // keyboard / mouse buttons
	evRel uint16 = 2 // relative movement (mouse)
	evAbs uint16 = 3 // absolute input (touchscreen, tablet)
)

// Config holds tunable idle parameters loaded from settings.
type Config struct {
	// LockTimeout is the duration of inactivity before the screen locks.
	// Zero disables idle locking.
	LockTimeout time.Duration
	// SuspendTimeout is the additional duration after locking before the
	// system suspends. Zero disables auto-suspend.
	SuspendTimeout time.Duration
}

// DefaultConfig returns the factory defaults (5 min lock, no auto-suspend).
func DefaultConfig() Config {
	return Config{
		LockTimeout:    5 * time.Minute,
		SuspendTimeout: 0,
	}
}

const (
	logindDest    = "org.freedesktop.login1"
	logindManager = "/org/freedesktop/login1"
	managerIface  = "org.freedesktop.login1.Manager"
	sessionIface  = "org.freedesktop.login1.Session"
)

// Service detects user inactivity and triggers locking and optional suspend.
type Service struct {
	bus  *bus.Bus
	conn *dbus.Conn // system bus; may be nil
	cfg  Config

	mu           sync.Mutex
	lastActivity time.Time
	locked       bool
}

// New creates the idle service. conn may be nil; logind integration is
// skipped when no system bus is available.
func New(b *bus.Bus, conn *dbus.Conn, cfg Config) *Service {
	return &Service{
		bus:          b,
		conn:         conn,
		cfg:          cfg,
		lastActivity: time.Now(),
	}
}

// UpdateConfig replaces the running configuration.
func (s *Service) UpdateConfig(cfg Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
}

// Run starts all monitoring goroutines and the idle check ticker.
// Blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	if s.conn != nil {
		go s.monitorLogind(ctx)
	}
	go s.monitorInputDevices(ctx)

	// When the lockscreen is dismissed, reset idle state.
	s.bus.Subscribe(bus.TopicScreenLock, func(e bus.Event) {
		if ls, ok := e.Data.(state.LockScreenState); ok && !ls.Locked {
			s.mu.Lock()
			s.locked = false
			s.lastActivity = time.Now()
			s.mu.Unlock()
		}
	})

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *Service) tick() {
	s.mu.Lock()
	cfg := s.cfg
	locked := s.locked
	since := time.Since(s.lastActivity)
	s.mu.Unlock()

	if cfg.LockTimeout == 0 {
		return
	}
	if !locked && since >= cfg.LockTimeout {
		s.doLock()
		return
	}
	if locked && cfg.SuspendTimeout > 0 && since >= cfg.LockTimeout+cfg.SuspendTimeout {
		log.Printf("[IDLE] suspending after lock timeout")
		go func() {
			if err := exec.Command("systemctl", "suspend").Run(); err != nil {
				log.Printf("[IDLE] suspend: %v", err)
			}
		}()
	}
}

func (s *Service) doLock() {
	s.mu.Lock()
	if s.locked {
		s.mu.Unlock()
		return
	}
	s.locked = true
	s.mu.Unlock()
	log.Printf("[IDLE] idle timeout reached — locking screen")
	s.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: true})
}

func (s *Service) recordActivity() {
	s.mu.Lock()
	s.lastActivity = time.Now()
	s.mu.Unlock()
}

// ── input device monitoring ───────────────────────────────────────────────────

func (s *Service) monitorInputDevices(ctx context.Context) {
	devices := findAllInputDevices()
	if len(devices) == 0 {
		log.Printf("[IDLE] no /dev/input/event* devices found — idle detection requires 'input' group membership")
		return
	}
	log.Printf("[IDLE] watching %d input device(s) for activity", len(devices))
	for _, dev := range devices {
		go s.readDevice(ctx, dev)
	}
}

func (s *Service) readDevice(ctx context.Context, path string) {
	f, err := os.Open(path)
	if err != nil {
		return // silently skip unreadable devices
	}
	defer f.Close()

	buf := make([]byte, 24)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := f.Read(buf)
			if err != nil || n != 24 {
				return
			}
			typ := binary.LittleEndian.Uint16(buf[16:18])
			if typ == evKey || typ == evRel || typ == evAbs {
				s.recordActivity()
			}
		}
	}
}

// findAllInputDevices returns paths to every event device listed in
// /proc/bus/input/devices that exists and is accessible.
func findAllInputDevices() []string {
	data, err := os.ReadFile("/proc/bus/input/devices")
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var devices []string

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var handlers string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			handlers = ""
			continue
		}
		if strings.HasPrefix(line, "H: Handlers=") {
			handlers = strings.TrimPrefix(line, "H: Handlers=")
		}
		if handlers == "" {
			continue
		}
		for _, field := range strings.Fields(handlers) {
			if !strings.HasPrefix(field, "event") {
				continue
			}
			path := filepath.Join("/dev/input", field)
			if seen[path] {
				continue
			}
			if _, err := os.Stat(path); err == nil {
				seen[path] = true
				devices = append(devices, path)
			}
		}
	}
	return devices
}

// ── logind integration ────────────────────────────────────────────────────────

func (s *Service) monitorLogind(ctx context.Context) {
	// PrepareForSleep(before bool) — lock before sleeping, re-check on wake.
	if err := s.conn.AddMatchSignal(
		dbus.WithMatchInterface(managerIface),
		dbus.WithMatchMember("PrepareForSleep"),
	); err != nil {
		log.Printf("[IDLE] logind PrepareForSleep match: %v", err)
	}

	// Session Lock/Unlock signals — interop with loginctl lock-session.
	session, err := resolveSession(s.conn)
	if err != nil {
		log.Printf("[IDLE] cannot resolve logind session: %v", err)
	}
	if session != "" {
		for _, member := range []string{"Lock", "Unlock"} {
			_ = s.conn.AddMatchSignal(
				dbus.WithMatchObjectPath(dbus.ObjectPath(session)),
				dbus.WithMatchMember(member),
			)
		}
	}

	ch := make(chan *dbus.Signal, 16)
	s.conn.Signal(ch)
	defer s.conn.RemoveSignal(ch)

	for {
		select {
		case <-ctx.Done():
			return
		case sig, ok := <-ch:
			if !ok {
				return
			}
			switch sig.Name {
			case managerIface + ".PrepareForSleep":
				if len(sig.Body) > 0 {
					if before, ok := sig.Body[0].(bool); ok {
						if before {
							// Going to sleep — lock now.
							s.doLock()
						} else {
							// Waking up — ensure lockscreen is visible.
							s.mu.Lock()
							wasLocked := s.locked
							s.mu.Unlock()
							if wasLocked {
								s.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: true})
							}
						}
					}
				}
			case sessionIface + ".Lock":
				s.doLock()
			case sessionIface + ".Unlock":
				s.mu.Lock()
				s.locked = false
				s.lastActivity = time.Now()
				s.mu.Unlock()
				s.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: false})
			}
		}
	}
}

func resolveSession(conn *dbus.Conn) (string, error) {
	if id := os.Getenv("XDG_SESSION_ID"); id != "" {
		return "/org/freedesktop/login1/session_" + id, nil
	}
	mgr := conn.Object(logindDest, logindManager)
	var path dbus.ObjectPath
	if err := mgr.Call(managerIface+".GetSessionByPID", 0, uint32(os.Getpid())).Store(&path); err != nil {
		return "", err
	}
	return string(path), nil
}
