// Package inputmode manages the global input mode (auto/tablet/desktop)
// and publishes an effective tablet-mode boolean for the rest of the shell.
//
// In "auto" mode the service combines evdev SW_TABLET_MODE (authoritative),
// logind's TabletMode property, and a physical-keyboard-activity heuristic
// to decide whether the OSK should auto-trigger.
//
// Priority: evdev SW_TABLET_MODE > logind TabletMode > keyboard heuristic.
package inputmode

import (
	"context"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/holoplot/go-evdev"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/dbusutil"
	"github.com/sonroyaalmerol/snry-shell/internal/settings"
)

const (
	logindProp          = "TabletMode"
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

	// evdev tablet mode state
	evdevTablet    bool
	evdevAvailable bool
}

// New creates the service. Touch device detection runs in Run().
func New(b *bus.Bus, conn dbusutil.DBusConn, cfg settings.Config, _ bool) *Service {
	s := &Service{
		bus:  b,
		conn: conn,
		mode: cfg.InputMode,
	}
	if s.mode == "" {
		s.mode = "auto"
	}
	return s
}

// Run starts evdev monitoring, logind monitoring, keyboard activity monitoring
// and the system-controls listener.  Blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	// Detect touch device availability up front.
	s.hasTouch = detectTouchDevice()

	go s.monitorLogind(ctx)
	go s.monitorEvdevSwitches(ctx)
	go s.monitorKeyboard(ctx)

	s.bus.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
		action, _ := e.Data.(string)
		if after, ok := strings.CutPrefix(action, "set-input-mode:"); ok {
			s.SetMode(after)
		}
	})

	s.bus.Subscribe(bus.TopicSettingsChanged, func(e bus.Event) {
		if cfg, ok := e.Data.(settings.Config); ok {
			changed := false
			s.mu.Lock()
			if s.mode != cfg.InputMode {
				s.mode = cfg.InputMode
				if s.mode == "" {
					s.mode = "auto"
				}
				changed = true
			}
			s.mu.Unlock()
			if changed {
				s.publish()
			}
		}
	})

	// Publish initial state immediately so late subscribers get the saved mode.
	s.publish()

	// Re-publish after monitors have had time to detect initial state.
	time.AfterFunc(500*time.Millisecond, func() { s.publish() })

	<-ctx.Done()
	return ctx.Err()
}

// SetMode changes the input mode, persists it, and republishes.
func (s *Service) SetMode(mode string) {
	s.mu.Lock()
	switch mode {
	case "auto", "tablet", "desktop":
		s.mode = mode
	default:
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	if err := s.persist(); err != nil {
		log.Printf("[inputmode] persist: %v", err)
	}
	s.publish()
}

// persist saves the current mode to the settings file.
func (s *Service) persist() error {
	cfg, err := settings.Load()
	if err != nil {
		return err
	}
	s.mu.Lock()
	cfg.InputMode = s.mode
	s.mu.Unlock()
	return settings.Save(cfg)
}

// publish snapshots all guarded fields under the lock, then publishes.
func (s *Service) publish() {
	s.mu.Lock()
	mode := s.mode
	evdevAvail := s.evdevAvailable
	evdevTab := s.evdevTablet
	logind := s.logindMode
	kb := s.kbActive
	touch := s.hasTouch
	s.mu.Unlock()

	tablet := false
	switch mode {
	case "tablet":
		tablet = true
	case "desktop":
		tablet = false
	case "auto":
		if evdevAvail {
			tablet = evdevTab
		} else {
			switch logind {
			case "enabled":
				tablet = true
			case "disabled":
				tablet = false
			default:
				tablet = !kb && touch
			}
		}
	}
	log.Printf("[inputmode] mode=%s evdev=%v(logind=%s) kb=%v touch=%v → tablet=%v",
		mode, evdevTab, logind, kb, touch, tablet)
	s.bus.Publish(bus.TopicTabletMode, tablet)
	s.bus.Publish(bus.TopicInputMode, mode)
}

// ── evdev switch monitor ─────────────────────────────────────────────────────

func (s *Service) monitorEvdevSwitches(ctx context.Context) {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		log.Printf("[inputmode] evdev enumeration: %v", err)
		return
	}

	for _, p := range paths {
		dev, err := evdev.OpenWithFlags(p.Path, os.O_RDONLY)
		if err != nil {
			continue
		}

		hasSW := slices.Contains(dev.CapableTypes(), evdev.EV_SW)
		if !hasSW {
			dev.Close()
			continue
		}

		hasTabletMode := slices.Contains(dev.CapableEvents(evdev.EV_SW), evdev.SW_TABLET_MODE)
		if !hasTabletMode {
			dev.Close()
			continue
		}

		log.Printf("[inputmode] found SW_TABLET_MODE device: %s (%s)", p.Name, p.Path)

		state, err := dev.State(evdev.EV_SW)
		if err != nil {
			log.Printf("[inputmode] evdev state read: %v", err)
		} else {
			s.mu.Lock()
			s.evdevAvailable = true
			s.evdevTablet = state[evdev.SW_TABLET_MODE]
			s.mu.Unlock()
			s.publish()
		}

		dev.NonBlock()
		go s.readSwitchEvents(ctx, dev)
		return
	}

	log.Printf("[inputmode] no SW_TABLET_MODE device found, using logind/heuristic fallback")
}

func (s *Service) readSwitchEvents(ctx context.Context, dev *evdev.InputDevice) {
	defer dev.Close()

	for {
		// NonBlock so ctx cancellation can interrupt the read loop.
		event, err := dev.ReadOne()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				// No event available right now; brief sleep to avoid busy-spin.
				time.Sleep(50 * time.Millisecond)
				continue
			}
		}
		if event.Type == evdev.EV_SW && event.Code == evdev.SW_TABLET_MODE {
			s.mu.Lock()
			s.evdevAvailable = true
			s.evdevTablet = event.Value != 0
			s.mu.Unlock()
			s.publish()
		}
	}
}

// ── logind monitor ──────────────────────────────────────────────────────────

func (s *Service) monitorLogind(ctx context.Context) {
	if s.conn == nil {
		log.Printf("[inputmode] no system bus, skipping logind monitor")
		return
	}
	session, err := s.resolveSession()
	if err != nil {
		log.Printf("[inputmode] cannot resolve session: %v", err)
		return
	}
	s.session = session

	ch := make(chan *dbus.Signal, 16)
	s.conn.Signal(ch)
	defer s.conn.RemoveSignal(ch)

	if err := s.conn.AddMatchSignal(dbus.WithMatchObjectPath(dbus.ObjectPath(session))); err != nil {
		log.Printf("[inputmode] AddMatchSignal: %v", err)
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
	realConn, ok := s.conn.(*dbusutil.RealConn)
	if !ok || realConn.Conn == nil {
		return "", fmt.Errorf("not a real connection")
	}
	path, err := dbusutil.GetSessionPath(realConn.Conn)
	if err != nil {
		return "", err
	}
	return string(path), nil
}

func (s *Service) queryLogind() {
	if s.conn == nil || s.session == "" {
		return
	}
	obj := s.conn.Object(dbusutil.LogindDest, dbus.ObjectPath(s.session))
	v, err := obj.GetProperty(dbusutil.LogindSession + "." + logindProp)
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

// ── keyboard activity monitor ──────────────────────────────────────────────

func (s *Service) monitorKeyboard(ctx context.Context) {
	devices := findPhysicalKeyboardDevices()
	if len(devices) == 0 {
		log.Printf("[inputmode] no physical keyboard devices found")
		return
	}
	log.Printf("[inputmode] monitoring %d keyboard device(s)", len(devices))

	for _, dev := range devices {
		dev.NonBlock()
		go s.readKeyboard(ctx, dev)
	}
}

func (s *Service) readKeyboard(ctx context.Context, dev *evdev.InputDevice) {
	defer dev.Close()

	for {
		// NonBlock so ctx cancellation can interrupt the read loop.
		event, err := dev.ReadOne()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(50 * time.Millisecond)
				continue
			}
		}
		if event.Type == evdev.EV_KEY && event.Value == 1 {
			s.onKeyboardActivity()
		}
	}
}

func (s *Service) onKeyboardActivity() {
	s.mu.Lock()
	s.kbActive = true
	if s.kbTimer != nil {
		s.kbTimer.Stop()
	}
	s.kbTimer = time.AfterFunc(kbInactivityTimeout, func() {
		s.mu.Lock()
		s.kbActive = false
		s.mu.Unlock()
		s.publish()
	})
	s.mu.Unlock()
	s.publish()
}

// ── evdev device helpers ────────────────────────────────────────────────────

func findPhysicalKeyboardDevices() []*evdev.InputDevice {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		log.Printf("[inputmode] evdev enumeration: %v", err)
		return nil
	}

	var devices []*evdev.InputDevice
	for _, p := range paths {
		if isVirtualKeyboard(p.Name) {
			continue
		}

		dev, err := evdev.OpenWithFlags(p.Path, os.O_RDONLY)
		if err != nil {
			continue
		}

		hasKey := slices.Contains(dev.CapableTypes(), evdev.EV_KEY)
		if !hasKey {
			dev.Close()
			continue
		}

		hasKeyA := slices.Contains(dev.CapableEvents(evdev.EV_KEY), 0x1e)
		if !hasKeyA {
			dev.Close()
			continue
		}

		devices = append(devices, dev)
	}
	return devices
}

func detectTouchDevice() bool {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return false
	}

	for _, p := range paths {
		dev, err := evdev.OpenWithFlags(p.Path, os.O_RDONLY)
		if err != nil {
			continue
		}

		hasABS := slices.Contains(dev.CapableTypes(), evdev.EV_ABS)
		if !hasABS {
			dev.Close()
			continue
		}

		if slices.Contains(dev.CapableEvents(evdev.EV_ABS), evdev.ABS_MT_POSITION_X) {
			dev.Close()
			return true
		}
		dev.Close()
	}
	return false
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
