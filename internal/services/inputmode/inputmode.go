// Package inputmode manages the global input mode (auto/tablet/desktop)
// and publishes an effective tablet-mode boolean for the rest of the shell.
//
// In "auto" mode the service uses a priority chain to decide tablet mode:
//
//  1. evdev SW_TABLET_MODE switch   (hardware switch on 2-in-1 devices)
//  2. IIO gravity sensor posture    (detects laptop vs studio/folded-back)
//  3. logind TabletMode property
//  4. keyboard presence heuristic   (has touch device but no physical keyboard)
package inputmode

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	logindProp           = "TabletMode"
	keyboardPollInterval = 2 * time.Second
	iioPollInterval      = 1 * time.Second
	iioDebounceCount     = 3
	iioDeviceDir         = "/sys/bus/iio/devices"
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

	mode        string // "auto", "tablet", "desktop"
	logindMode  string // "enabled", "disabled", "indeterminate"
	hasKeyboard bool
	hasTouch    bool
	session     string

	// evdev tablet mode state
	evdevTablet    bool
	evdevAvailable bool

	// IIO gravity sensor state
	iioTablet    bool
	iioAvailable bool
	iioPath      string
}

// New creates the service. Device detection runs in Run().
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

// Run starts all monitors and the system-controls listener.
// Blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	s.hasTouch = detectTouchDevice()
	s.hasKeyboard = hasPhysicalKeyboard()
	s.resolveIIODevice()

	go s.monitorLogind(ctx)
	go s.monitorEvdevSwitches(ctx)
	go s.monitorKeyboardPresence(ctx)
	if s.iioAvailable {
		go s.monitorIIOGravity(ctx)
	}

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
	iioAvail := s.iioAvailable
	iioTab := s.iioTablet
	logind := s.logindMode
	kb := s.hasKeyboard
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
		} else if iioAvail {
			tablet = iioTab
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
	log.Printf("[inputmode] mode=%s evdev=%v iio=%v(logind=%s) kb=%v touch=%v → tablet=%v",
		mode, evdevTab, iioTab, logind, kb, touch, tablet)
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

	log.Printf("[inputmode] no SW_TABLET_MODE device found")
}

func (s *Service) readSwitchEvents(ctx context.Context, dev *evdev.InputDevice) {
	defer dev.Close()

	// Poll for events using a context-aware ticker instead of a tight
	// 50ms busy-loop. Switch events are rare (only on tablet mode change)
	// so a 200ms poll interval has negligible latency while using far
	// less CPU than continuous spinning.
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		// Drain all pending events.
		for {
			event, err := dev.ReadOne()
			if err != nil {
				break
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

// ── IIO gravity sensor monitor ───────────────────────────────────────────────

// resolveIIODevice searches /sys/bus/iio/devices/ for a gravity sensor.
func (s *Service) resolveIIODevice() {
	entries, err := os.ReadDir(iioDeviceDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(iioDeviceDir, e.Name(), "name"))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == "gravity" {
			s.iioPath = filepath.Join(iioDeviceDir, e.Name())
			s.iioAvailable = true
			log.Printf("[inputmode] found IIO gravity sensor: %s", s.iioPath)
			return
		}
	}
	log.Printf("[inputmode] no IIO gravity sensor found")
}

// monitorIIOGravity polls the gravity sensor to detect device posture.
// When the Z-axis gravity dominates over Y-axis, the screen is folded
// back (studio/tablet mode). Uses debounce to avoid false triggers.
func (s *Service) monitorIIOGravity(ctx context.Context) {
	if tablet, ok := s.readGravityPosture(); ok {
		s.mu.Lock()
		s.iioTablet = tablet
		s.mu.Unlock()
		s.publish()
	}

	debounceCount := 0
	var lastCandidate bool

	ticker := time.NewTicker(iioPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tablet, ok := s.readGravityPosture()
			if !ok {
				continue
			}
			if tablet == lastCandidate {
				debounceCount++
			} else {
				lastCandidate = tablet
				debounceCount = 1
			}
			if debounceCount >= iioDebounceCount {
				s.mu.Lock()
				if s.iioTablet != tablet {
					s.iioTablet = tablet
					s.mu.Unlock()
					s.publish()
				} else {
					s.mu.Unlock()
				}
			}
		}
	}
}

// readGravityPosture reads the gravity sensor and returns true if the
// device appears to be in tablet/studio posture (Z-axis dominant).
func (s *Service) readGravityPosture() (tablet bool, ok bool) {
	yRaw, err := readIntFile(filepath.Join(s.iioPath, "in_gravity_y_raw"))
	if err != nil {
		return false, false
	}
	zRaw, err := readIntFile(filepath.Join(s.iioPath, "in_gravity_z_raw"))
	if err != nil {
		return false, false
	}
	yAbs := abs64(yRaw)
	zAbs := abs64(zRaw)
	// Tablet/studio: Z-axis gravity dominates (screen facing up with
	// keyboard folded behind) vs Y-axis (screen upright in laptop mode).
	return zAbs > yAbs, true
}

// ── keyboard presence monitor ───────────────────────────────────────────────

// monitorKeyboardPresence periodically rescans for physical keyboard devices.
// This catches hotplug events like a Surface Type Cover being attached/detached.
func (s *Service) monitorKeyboardPresence(ctx context.Context) {
	ticker := time.NewTicker(keyboardPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			kb := hasPhysicalKeyboard()
			s.mu.Lock()
			changed := s.hasKeyboard != kb
			if changed {
				s.hasKeyboard = kb
			}
			s.mu.Unlock()
			if changed {
				s.publish()
			}
		}
	}
}

// ── evdev device helpers ────────────────────────────────────────────────────

// hasPhysicalKeyboard reports whether at least one non-virtual physical keyboard
// device is currently connected.
func hasPhysicalKeyboard() bool {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return false
	}

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
		dev.Close()
		if hasKeyA {
			return true
		}
	}
	return false
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

// ── sysfs helpers ───────────────────────────────────────────────────────────

func readIntFile(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// init ensures the surface_aggregator_tabletsw module is loaded on Surface devices.
func init() {
	if _, err := os.Stat("/sys/bus/surface_aggregator"); err == nil {
		if err := exec.Command("modprobe", "surface_aggregator_tabletsw").Run(); err != nil {
			log.Printf("[inputmode] modprobe surface_aggregator_tabletsw: %v", err)
		}
	}
}
