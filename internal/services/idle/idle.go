// Package idle monitors user inactivity and triggers screen locking and
// optional system suspend. It replaces the functionality of hypridle.
package idle

import (
	"context"
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
	LockTimeout           time.Duration
	IdleDisplayOffTimeout time.Duration
	LockDisplayOffTimeout time.Duration
	SuspendTimeout        time.Duration
}

// DefaultConfig returns the factory defaults.
func DefaultConfig() Config {
	return Config{
		LockTimeout:           5 * time.Minute,
		IdleDisplayOffTimeout: 2 * time.Minute,
		LockDisplayOffTimeout: 30 * time.Second,
		SuspendTimeout:        0,
	}
}

// Service detects user inactivity and triggers locking and optional suspend.
type Service struct {
	bus  *bus.Bus
	conn dbusutil.DBusConn
	cfg  Config

	mu          sync.Mutex
	locked      bool
	displayOff  bool
	idleStarted time.Time
	inhibited   bool

	// Wayland fields
	waylandMu       sync.Mutex
	display         *client.Display
	manager         *protocol.ExtIdleNotifierV1
	seat            *client.Seat
	lockNotif       *protocol.ExtIdleNotificationV1
	displayOffNotif *protocol.ExtIdleNotificationV1
}

// New creates the idle service.
func New(b *bus.Bus, conn dbusutil.DBusConn, cfg Config) *Service {
	return &Service{
		bus:  b,
		conn: conn,
		cfg:  cfg,
	}
}

// NewWithDefaults creates the idle service with a system bus connection
// and default configuration.
func NewWithDefaults(b *bus.Bus) *Service {
	sysConn, err := dbus.ConnectSystemBus()
	if err != nil {
		return &Service{bus: b, cfg: DefaultConfig()}
	}
	return &Service{bus: b, conn: dbusutil.NewRealConn(sysConn), cfg: DefaultConfig()}
}

// UpdateConfig replaces the running configuration and recreates timers.
func (s *Service) UpdateConfig(cfg Config) {
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()

	s.waylandMu.Lock()
	defer s.waylandMu.Unlock()
	if s.manager != nil && s.seat != nil {
		log.Printf("[idle] config updated, recreating timers")
		s.recreateTimers()
	}
}

func (s *Service) recreateTimers() {
	// Cleanup existing
	if s.lockNotif != nil {
		s.lockNotif.Destroy()
		s.lockNotif = nil
	}
	if s.displayOffNotif != nil {
		s.displayOffNotif.Destroy()
		s.displayOffNotif = nil
	}

	s.mu.Lock()
	cfg := s.cfg
	locked := s.locked
	s.mu.Unlock()

	// 1. Lock Timer (only if not already locked)
	if !locked && cfg.LockTimeout > 0 {
		ms := uint32(cfg.LockTimeout.Milliseconds())
		notif, err := s.manager.GetIdleNotification(ms, s.seat)
		if err == nil {
			notif.SetIdledHandler(func(protocol.ExtIdleNotificationV1IdledEvent) {
				s.doLock()
			})
			s.lockNotif = notif
		}
	}

	// 2. Display Off Timer
	displayTimeout := cfg.IdleDisplayOffTimeout
	if locked {
		displayTimeout = cfg.LockDisplayOffTimeout
	}

	if displayTimeout > 0 {
		ms := uint32(displayTimeout.Milliseconds())
		notif, err := s.manager.GetIdleNotification(ms, s.seat)
		if err == nil {
			notif.SetIdledHandler(func(protocol.ExtIdleNotificationV1IdledEvent) {
				s.setDisplay(false)
			})
			notif.SetResumedHandler(func(protocol.ExtIdleNotificationV1ResumedEvent) {
				s.setDisplay(true)
			})
			s.displayOffNotif = notif
		}
	}
}

func (s *Service) setDisplay(on bool) {
	s.mu.Lock()
	if s.displayOff == !on {
		s.mu.Unlock()
		return
	}
	s.displayOff = !on
	s.mu.Unlock()

	if on {
		log.Printf("[idle] turning display ON")
		exec.Command("hyprctl", "dispatch", "dpms", "on").Run()
	} else {
		log.Printf("[idle] turning display OFF")
		if err := exec.Command("hyprctl", "dispatch", "dpms", "off").Run(); err != nil {
			exec.Command("xset", "dpms", "force", "off").Run()
		}
	}
}

// Run starts monitoring.
func (s *Service) Run(ctx context.Context) error {
	if realConn, ok := s.conn.(*dbusutil.RealConn); ok && realConn.Conn != nil {
		go s.monitorLogind(ctx)
		if err := RegisterScreenSaver(realConn.Conn, NewScreenSaver(s.bus)); err != nil {
			log.Printf("[idle] screensaver dbus: %v", err)
		}
	}

	go s.waylandLoop(ctx)

	// Centralized lock state tracking.
	s.bus.Subscribe(bus.TopicScreenLock, func(e bus.Event) {
		if ls, ok := e.Data.(state.LockScreenState); ok {
			s.mu.Lock()
			changed := s.locked != ls.Locked
			s.locked = ls.Locked
			if ls.Locked {
				s.idleStarted = time.Now()
			} else {
				s.idleStarted = time.Time{}
			}
			s.mu.Unlock()

			if changed {
				s.waylandMu.Lock()
				if s.manager != nil {
					s.recreateTimers()
				}
				s.waylandMu.Unlock()
			}
		}
	})

	// Inhibition.
	s.bus.Subscribe(bus.TopicIdleInhibit, func(e bus.Event) {
		if active, ok := e.Data.(bool); ok {
			s.mu.Lock()
			s.inhibited = active
			s.mu.Unlock()
			log.Printf("[idle] inhibition: %v", active)
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
	started := s.idleStarted
	inhibited := s.inhibited
	s.mu.Unlock()

	if inhibited || !locked || started.IsZero() {
		return
	}

	elapsed := time.Since(started)
	if cfg.SuspendTimeout > 0 && elapsed >= cfg.SuspendTimeout {
		s.mu.Lock()
		s.idleStarted = time.Time{}
		s.mu.Unlock()
		log.Printf("[idle] suspending system")
		go func() {
			if realConn, ok := s.conn.(*dbusutil.RealConn); ok && realConn.Conn != nil {
				dbusutil.LogindSuspend(realConn.Conn)
			}
		}()
	}
}

func (s *Service) waylandLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := s.initAndDispatch(ctx); err != nil {
				log.Printf("[idle] wayland error: %v, retrying in 5s", err)
				s.cleanupWayland()
				time.Sleep(5 * time.Second)
			}
		}
	}
}

func (s *Service) cleanupWayland() {
	s.waylandMu.Lock()
	defer s.waylandMu.Unlock()
	if s.lockNotif != nil {
		s.lockNotif = nil
	}
	if s.displayOffNotif != nil {
		s.displayOffNotif = nil
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

	var extIdleName, extIdleVer, seatName, seatVer uint32
	registry.SetGlobalHandler(func(e client.RegistryGlobalEvent) {
		switch e.Interface {
		case "ext_idle_notifier_v1":
			extIdleName, extIdleVer = e.Name, e.Version
		case "wl_seat":
			if seatName == 0 {
				seatName, seatVer = e.Name, e.Version
			}
		}
	})

	if err := waylandutil.Roundtrip(display); err != nil {
		display.Destroy()
		return err
	}

	s.waylandMu.Lock()
	s.display = display
	s.manager = protocol.NewExtIdleNotifierV1(display.Context())
	waylandutil.FixedBind(registry, extIdleName, "ext_idle_notifier_v1", extIdleVer, s.manager)
	s.seat = client.NewSeat(display.Context())
	waylandutil.FixedBind(registry, seatName, "wl_seat", seatVer, s.seat)
	s.waylandMu.Unlock()

	if err := waylandutil.Roundtrip(display); err != nil {
		display.Destroy()
		return err
	}

	s.waylandMu.Lock()
	s.recreateTimers()
	s.waylandMu.Unlock()

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

func (s *Service) doLock() {
	s.mu.Lock()
	if s.locked || s.inhibited {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	log.Printf("[idle] idle lock triggered")
	s.bus.Publish(bus.TopicScreenLock, state.LockScreenState{Locked: true})
}

func (s *Service) monitorLogind(ctx context.Context) {
	var rawConn *dbus.Conn
	if realConn, ok := s.conn.(*dbusutil.RealConn); ok && realConn.Conn != nil {
		rawConn = realConn.Conn
	}

	if err := s.conn.AddMatchSignal(
		dbus.WithMatchInterface(dbusutil.LogindManager),
		dbus.WithMatchMember("PrepareForSleep"),
	); err != nil {
		log.Printf("[idle] PrepareForSleep match: %v", err)
	}

	session, _ := dbusutil.GetSessionPath(rawConn)
	if session != "" {
		_ = s.conn.AddMatchSignal(dbus.WithMatchObjectPath(session), dbus.WithMatchInterface(dbusutil.LogindSession), dbus.WithMatchMember("Lock"))
		_ = s.conn.AddMatchSignal(dbus.WithMatchObjectPath(session), dbus.WithMatchInterface(dbusutil.LogindSession), dbus.WithMatchMember("Unlock"))
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
				if active, ok := sig.Body[0].(bool); ok && active {
					s.doLock()
				}
			case dbusutil.LogindSession + ".Lock":
				s.doLock()
			case dbusutil.LogindSession + ".Unlock":
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
	ss.bus.Publish(bus.TopicIdleInhibit, true)
	return id, nil
}

func (ss *ScreenSaver) UnInhibit(id uint32) *dbus.Error {
	ss.mu.Lock()
	delete(ss.inhibitors, id)
	count := len(ss.inhibitors)
	ss.mu.Unlock()
	if count == 0 {
		ss.bus.Publish(bus.TopicIdleInhibit, false)
	}
	return nil
}

func (ss *ScreenSaver) Lock() *dbus.Error {
	ss.bus.Publish(bus.TopicSystemControls, "toggle-lock")
	return nil
}

func (ss *ScreenSaver) SimulateUserActivity() *dbus.Error { return nil }
func (ss *ScreenSaver) GetActive() (bool, *dbus.Error)    { return false, nil }

func RegisterScreenSaver(conn *dbus.Conn, ss *ScreenSaver) error {
	if err := conn.Export(ss, screenSaverPath, screenSaverIface); err != nil {
		return err
	}
	_, err := conn.RequestName(screenSaverName, dbus.NameFlagReplaceExisting)
	return err
}
