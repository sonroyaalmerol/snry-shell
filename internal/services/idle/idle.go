// Package idle monitors user inactivity and triggers screen locking and
// optional system suspend. It replaces the functionality of hypridle.
//
// Activity is detected via the ext-idle-notify-v1 Wayland protocol,
// which is standard on Hyprland setups.
//
// Logind integration: PrepareForSleep locks the screen before suspend;
// the session Lock/Unlock D-Bus signals make loginctl lock-session
// interoperable with the shell lockscreen.
package idle

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/rajveermalviya/go-wayland/wayland/client"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/dbusutil"
	protocol "github.com/sonroyaalmerol/snry-shell/internal/services/idle/protocol"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
	"github.com/sonroyaalmerol/snry-shell/internal/waylandutil"
)

// Config holds tunable idle parameters loaded from settings.
type Config struct {
	// LockTimeout is the duration of inactivity before the screen locks.
	// Zero disables idle locking.
	LockTimeout time.Duration
	// DisplayOffTimeout is the additional duration after locking before the
	// screen turns off. Zero disables display off.
	DisplayOffTimeout time.Duration
	// SuspendTimeout is the additional duration after locking before the
	// system suspends. Zero disables auto-suspend.
	SuspendTimeout time.Duration
}

// DefaultConfig returns the factory defaults.
func DefaultConfig() Config {
	return Config{
		LockTimeout:       5 * time.Minute,
		DisplayOffTimeout: 30 * time.Second,
		SuspendTimeout:    0,
	}
}

// Service detects user inactivity and triggers locking and optional suspend.
type Service struct {
	bus  *bus.Bus
	conn *dbus.Conn // system bus; may be nil
	cfg  Config

	mu          sync.Mutex
	locked      bool
	displayOff  bool
	idleStarted time.Time
	inhibited   bool // true if D-Bus or manual inhibition is active

	// Wayland fields
	waylandMu     sync.Mutex
	display       *client.Display
	manager       *protocol.ExtIdleNotifierV1
	seat          *client.Seat
	activeNotif   *protocol.ExtIdleNotificationV1
	activeTimeout uint32
}

// New creates the idle service. conn may be nil; logind integration is
// skipped when no system bus is available.
func New(b *bus.Bus, conn *dbus.Conn, cfg Config) *Service {
	return &Service{
		bus:  b,
		conn: conn,
		cfg:  cfg,
	}
}

// UpdateConfig replaces the running configuration.
func (s *Service) UpdateConfig(cfg Config) {
	s.mu.Lock()
	oldTimeout := s.cfg.LockTimeout
	s.cfg = cfg
	s.mu.Unlock()

	// Only recreate if the timeout value changed.
	if cfg.LockTimeout != oldTimeout {
		s.waylandMu.Lock()
		defer s.waylandMu.Unlock()
		if s.manager != nil && s.seat != nil {
			log.Printf("[IDLE] config updated, recreating notification")
			s.recreateNotification(cfg.LockTimeout)
		}
	}
}

func (s *Service) recreateNotification(timeout time.Duration) {
	if s.activeNotif != nil {
		s.activeNotif.Destroy()
		s.activeNotif = nil
	}
	s.activeTimeout = 0

	ms := uint32(timeout.Milliseconds())
	if ms == 0 {
		return
	}

	notif, err := s.manager.GetIdleNotification(ms, s.seat)
	if err != nil {
		log.Printf("[IDLE] GetIdleNotification failed: %v", err)
		return
	}

	notif.SetIdledHandler(func(protocol.ExtIdleNotificationV1IdledEvent) {
		s.doLock()
	})

	notif.SetResumedHandler(func(protocol.ExtIdleNotificationV1ResumedEvent) {
		s.mu.Lock()
		s.locked = false
		s.idleStarted = time.Time{}
		wasDisplayOff := s.displayOff
		s.displayOff = false
		s.mu.Unlock()
		if wasDisplayOff {
			log.Printf("[IDLE] turning display on")
			exec.Command("hyprctl", "dispatch", "dpms", "on").Run()
		}
		log.Printf("[IDLE] resumed activity")
	})

	s.activeNotif = notif
	s.activeTimeout = ms
}

// Run starts all monitoring goroutines and the idle check ticker.
// Blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	if s.conn != nil {
		go s.monitorLogind(ctx)
		// Register D-Bus ScreenSaver interface to bridge non-Wayland inhibitors.
		if err := RegisterScreenSaver(s.conn, NewScreenSaver(s.bus)); err != nil {
			log.Printf("[IDLE] screensaver dbus: %v", err)
		}
	}

	go s.waylandLoop(ctx)

	// When the lockscreen is dismissed, reset idle state.
	s.bus.Subscribe(bus.TopicScreenLock, func(e bus.Event) {
		if ls, ok := e.Data.(state.LockScreenState); ok && !ls.Locked {
			s.mu.Lock()
			s.locked = false
			s.idleStarted = time.Time{}
			s.mu.Unlock()
		}
	})

	// Watch for manual/dbus inhibition topic.
	s.bus.Subscribe(bus.TopicIdleInhibit, func(e bus.Event) {
		if active, ok := e.Data.(bool); ok {
			s.mu.Lock()
			s.inhibited = active
			if active {
				s.idleStarted = time.Time{}
			}
			s.mu.Unlock()
			log.Printf("[IDLE] external inhibition: %v", active)
		}
	})

	// Watch for DND (mapped to manual idle off in UI).
	s.bus.Subscribe(bus.TopicDND, func(e bus.Event) {
		if active, ok := e.Data.(bool); ok {
			s.bus.Publish(bus.TopicIdleInhibit, active)
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

func (s *Service) waylandLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := s.initAndDispatch(ctx); err != nil {
				log.Printf("[IDLE] wayland connection error: %v, retrying in 5s", err)
				s.cleanupWayland()
				time.Sleep(5 * time.Second)
			}
		}
	}
}

func (s *Service) cleanupWayland() {
	s.waylandMu.Lock()
	defer s.waylandMu.Unlock()
	if s.activeNotif != nil {
		s.activeNotif = nil
	}
	if s.display != nil {
		s.display.Context().Close()
		s.display = nil
	}
	s.manager = nil
	s.seat = nil
}

func (s *Service) initAndDispatch(ctx context.Context) error {
	display, err := client.Connect("")
	if err != nil {
		return err
	}

	registry, err := display.GetRegistry()
	if err != nil {
		display.Destroy()
		return err
	}

	var (
		extIdleName uint32
		extIdleVer  uint32
		seatName    uint32
		seatVer     uint32
	)

	registry.SetGlobalHandler(func(e client.RegistryGlobalEvent) {
		switch e.Interface {
		case "ext_idle_notifier_v1":
			extIdleName = e.Name
			extIdleVer = e.Version
		case "wl_seat":
			if seatName == 0 {
				seatName = e.Name
				seatVer = e.Version
			}
		}
	})

	if err := waylandutil.Roundtrip(display); err != nil {
		display.Destroy()
		return err
	}

	if extIdleName == 0 || seatName == 0 {
		display.Destroy()
		return fmt.Errorf("required interfaces (ext_idle_notifier_v1, wl_seat) not found")
	}

	s.waylandMu.Lock()
	s.display = display
	s.manager = protocol.NewExtIdleNotifierV1(display.Context())
	if err := waylandutil.FixedBind(registry, extIdleName, "ext_idle_notifier_v1", extIdleVer, s.manager); err != nil {
		s.waylandMu.Unlock()
		display.Destroy()
		return err
	}

	s.seat = client.NewSeat(display.Context())
	if err := waylandutil.FixedBind(registry, seatName, "wl_seat", seatVer, s.seat); err != nil {
		s.waylandMu.Unlock()
		display.Destroy()
		return err
	}

	s.waylandMu.Unlock()
	if err := waylandutil.Roundtrip(display); err != nil {
		display.Destroy()
		return err
	}

	s.waylandMu.Lock()
	s.mu.Lock()
	timeout := s.cfg.LockTimeout
	s.mu.Unlock()
	s.recreateNotification(timeout)
	s.waylandMu.Unlock()

	log.Printf("[IDLE] watching for ext-idle-notify-v1 events")

	d := display
	go func() {
		<-ctx.Done()
		d.Context().Close()
	}()

	for {
		s.waylandMu.Lock()
		if s.display == nil {
			s.waylandMu.Unlock()
			return nil
		}
		dispCtx := s.display.Context()
		s.waylandMu.Unlock()

		if err := dispCtx.Dispatch(); err != nil {
			return err
		}
	}
}

func (s *Service) tick() {
	s.mu.Lock()
	cfg := s.cfg
	locked := s.locked
	started := s.idleStarted
	displayOff := s.displayOff
	inhibited := s.inhibited
	s.mu.Unlock()

	// If inhibited, we manually prevent idling by "touching" the state.
	if inhibited {
		return
	}

	if !locked || started.IsZero() {
		return
	}

	elapsed := time.Since(started)

	if cfg.DisplayOffTimeout > 0 && !displayOff && elapsed >= cfg.DisplayOffTimeout {
		log.Printf("[IDLE] turning display off after timeout")
		s.mu.Lock()
		s.displayOff = true
		s.mu.Unlock()
		go func() {
			if err := exec.Command("hyprctl", "dispatch", "dpms", "off").Run(); err != nil {
				// Fallback to generic DPMS if hyprctl fails or isn't available
				exec.Command("xset", "dpms", "force", "off").Run()
			}
		}()
	}

	if cfg.SuspendTimeout > 0 && elapsed >= cfg.SuspendTimeout {
		log.Printf("[IDLE] suspending after suspend timeout")
		s.mu.Lock()
		s.idleStarted = time.Time{}
		s.mu.Unlock()
		go dbusutil.LogindSuspend(s.conn)
	}
}

func (s *Service) doLock() {
	s.mu.Lock()
	if s.locked || s.inhibited {
		s.mu.Unlock()
		return
	}
	s.locked = true
	s.idleStarted = time.Now()
	s.mu.Unlock()
	log.Printf("[IDLE] idle timeout reached — locking screen")
	s.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: true})
}

// ── logind integration ────────────────────────────────────────────────────────

func (s *Service) monitorLogind(ctx context.Context) {
	if err := s.conn.AddMatchSignal(
		dbus.WithMatchInterface(dbusutil.LogindManager),
		dbus.WithMatchMember("PrepareForSleep"),
	); err != nil {
		log.Printf("[IDLE] logind PrepareForSleep match: %v", err)
	}

	session, err := dbusutil.GetSessionPath(s.conn)
	if err != nil {
		log.Printf("[IDLE] cannot resolve logind session: %v", err)
	}
	if session != "" {
		for _, member := range []string{"Lock", "Unlock"} {
			_ = s.conn.AddMatchSignal(
				dbus.WithMatchObjectPath(session),
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
			case dbusutil.LogindManager + ".PrepareForSleep":
				if len(sig.Body) > 0 {
					if before, ok := sig.Body[0].(bool); ok {
						if before {
							s.doLock()
						} else {
							s.mu.Lock()
							wasLocked := s.locked
							s.mu.Unlock()
							if wasLocked {
								s.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: true})
							}
						}
					}
				}
			case dbusutil.LogindSession + ".Lock":
				s.doLock()
			case dbusutil.LogindSession + ".Unlock":
				s.mu.Lock()
				s.locked = false
				s.idleStarted = time.Time{}
				s.mu.Unlock()
				s.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: false})
			}
		}
	}
}

// ── ScreenSaver D-Bus implementation ──────────────────────────────────────────

const (
	screenSaverName  = "org.freedesktop.ScreenSaver"
	screenSaverPath  = "/org/freedesktop/ScreenSaver"
	screenSaverIface = "org.freedesktop.ScreenSaver"
)

type ScreenSaver struct {
	bus        *bus.Bus
	inhibitors map[uint32]string
	mu         sync.Mutex
	id         uint32
}

func NewScreenSaver(b *bus.Bus) *ScreenSaver {
	return &ScreenSaver{
		bus:        b,
		inhibitors: make(map[uint32]string),
	}
}

func (ss *ScreenSaver) Inhibit(appName string, reason string) (uint32, *dbus.Error) {
	ss.mu.Lock()
	ss.id++
	id := ss.id
	ss.inhibitors[id] = appName
	ss.mu.Unlock()

	log.Printf("[IDLE] DBus inhibit from %s: %s (id=%d)", appName, reason, id)
	ss.bus.Publish(bus.TopicIdleInhibit, true)
	return id, nil
}

func (ss *ScreenSaver) UnInhibit(id uint32) *dbus.Error {
	ss.mu.Lock()
	delete(ss.inhibitors, id)
	count := len(ss.inhibitors)
	ss.mu.Unlock()

	log.Printf("[IDLE] DBus uninhibit (id=%d), remaining=%d", id, count)
	if count == 0 {
		ss.bus.Publish(bus.TopicIdleInhibit, false)
	}
	return nil
}

func (ss *ScreenSaver) Lock() *dbus.Error {
	ss.bus.Publish(bus.TopicSystemControls, "toggle-lock")
	return nil
}

func (ss *ScreenSaver) SimulateUserActivity() *dbus.Error {
	return nil
}

func (ss *ScreenSaver) GetActive() (bool, *dbus.Error) {
	return false, nil
}

func RegisterScreenSaver(conn *dbus.Conn, ss *ScreenSaver) error {
	if err := conn.Export(ss, screenSaverPath, screenSaverIface); err != nil {
		return fmt.Errorf("export screensaver: %w", err)
	}
	reply, err := conn.RequestName(screenSaverName, dbus.NameFlagReplaceExisting)
	if err != nil {
		return fmt.Errorf("request screensaver name: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		log.Printf("[IDLE] warning: could not own %s, reply=%d", screenSaverName, reply)
	}
	return nil
}
