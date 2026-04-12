package mpris

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/dbusutil"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const (
	mprisPrefix     = "org.mpris.MediaPlayer2."
	playerIface     = "org.mpris.MediaPlayer2.Player"
	propertiesIface = "org.freedesktop.DBus.Properties"
)

type Service struct {
	conn          dbusutil.DBusConn
	bus           *bus.Bus
	playerNameMap map[string]string
	activePlayer  atomic.Pointer[string]
}

func New(conn dbusutil.DBusConn, b *bus.Bus) *Service {
	return &Service{conn: conn, bus: b, playerNameMap: make(map[string]string)}
}

func NewWithDefaults(b *bus.Bus) *Service {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return &Service{bus: b, playerNameMap: make(map[string]string)}
	}
	return &Service{conn: dbusutil.NewRealConn(conn), bus: b, playerNameMap: make(map[string]string)}
}

func (s *Service) Run(ctx context.Context) error {
	s.refreshNameMap()
	ch := make(chan *dbus.Signal, 16)
	s.conn.Signal(ch)
	defer s.conn.RemoveSignal(ch)

	// Subscribe to PropertiesChanged on all MPRIS players.
	s.conn.BusObject().Call(
		"org.freedesktop.DBus.AddMatch", 0,
		"type='signal',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged'",
	)

	// Subscribe to Seeked signal for position updates without polling.
	s.conn.BusObject().Call(
		"org.freedesktop.DBus.AddMatch", 0,
		"type='signal',interface='org.mpris.MediaPlayer2.Player',member='Seeked'",
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig, ok := <-ch:
			if !ok {
				return nil
			}
			s.handleSignal(sig)
		}
	}
}

func (s *Service) handleSignal(sig *dbus.Signal) {
	switch sig.Name {
	case "org.freedesktop.DBus.Properties.PropertiesChanged":
		s.handlePropertiesChanged(sig)
	case "org.mpris.MediaPlayer2.Player.Seeked":
		s.handleSeeked(sig)
	}
}

func (s *Service) handlePropertiesChanged(sig *dbus.Signal) {
	if len(sig.Body) < 2 {
		return
	}
	iface, ok := sig.Body[0].(string)
	if !ok || iface != playerIface {
		return
	}

	sender := string(sig.Sender)

	changed, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return
	}

	// Resolve well-known name from unique name.
	wellKnown := s.resolvePlayerName(sender)

	player := s.parseChangedProps(wellKnown, changed)

	// Track the most recently active (playing) player.
	if v, ok := changed["PlaybackStatus"]; ok {
		if status, ok := v.Value().(string); ok && status == "Playing" {
			s.activePlayer.Store(&wellKnown)
		}
	}

	s.bus.Publish(bus.TopicMedia, player)
}

func (s *Service) handleSeeked(sig *dbus.Signal) {
	if len(sig.Body) < 1 {
		return
	}
	pos, ok := sig.Body[0].(int64)
	if !ok {
		return
	}
	wellKnown := s.resolvePlayerName(string(sig.Sender))
	s.bus.Publish(bus.TopicMediaTick, state.MediaTick{
		PlayerName: wellKnown,
		Position:   float64(pos) / 1e6,
		At:         time.Now(),
	})
}

func (s *Service) parseChangedProps(sender string, props map[string]dbus.Variant) state.MediaPlayer {
	mp := state.MediaPlayer{PlayerName: sender}

	if v, ok := props["PlaybackStatus"]; ok {
		if status, ok := v.Value().(string); ok {
			mp.Playing = status == "Playing"
		}
	}
	if v, ok := props["Metadata"]; ok {
		if meta, ok := v.Value().(map[string]dbus.Variant); ok {
			if t, ok := meta["xesam:title"]; ok {
				mp.Title, _ = t.Value().(string)
			}
			if a, ok := meta["xesam:artist"]; ok {
				switch v := a.Value().(type) {
				case []string:
					if len(v) > 0 {
						mp.Artist = v[0]
					}
				case string:
					mp.Artist = v
				}
			}
			if art, ok := meta["mpris:artUrl"]; ok {
				if url, ok := art.Value().(string); ok {
					mp.ArtPath = resolveArtURL(url)
				}
			}
			if len, ok := meta["mpris:length"]; ok {
				if micros, ok := len.Value().(int64); ok {
					mp.Duration = float64(micros) / 1e6
				}
			}
		}
	}
	if v, ok := props["CanGoNext"]; ok {
		mp.CanNext, _ = v.Value().(bool)
	}
	if v, ok := props["CanGoPrevious"]; ok {
		mp.CanPrev, _ = v.Value().(bool)
	}
	return mp
}

// resolveArtURL converts a MPRIS art URL to a local file path.
func resolveArtURL(url string) string {
	if after, ok := strings.CutPrefix(url, "file://"); ok {
		return after
	}
	return url
}

// refreshNameMap queries the DBus for all MPRIS players and builds a
// unique-name → well-known-name mapping.
func (s *Service) refreshNameMap() {
	var names []string
	v, err := s.conn.BusObject().GetProperty("org.freedesktop.DBus.ListNames")
	if err != nil {
		return
	}
	if arr, ok := v.Value().([]string); ok {
		for _, name := range arr {
			if strings.HasPrefix(name, mprisPrefix) {
				names = append(names, name)
			}
		}
	}
	for _, wellKnown := range names {
		call := s.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, wellKnown)
		if call.Err == nil {
			if unique, ok := call.Body[0].(string); ok {
				s.playerNameMap[unique] = wellKnown
			}
		}
	}
}

// resolvePlayerName returns the well-known MPRIS name for a unique bus name.
func (s *Service) resolvePlayerName(sender string) string {
	if wellKnown, ok := s.playerNameMap[sender]; ok {
		return wellKnown
	}
	s.refreshNameMap()
	if wellKnown, ok := s.playerNameMap[sender]; ok {
		return wellKnown
	}
	return sender
}

// SeekTo seeks to the given position (in seconds) on the specified player.
func (s *Service) SeekTo(playerBusName string, positionSeconds float64) error {
	obj := s.conn.Object(playerBusName, "/org/mpris/MediaPlayer2")
	pos := int64(positionSeconds * 1e6)
	return obj.Call(playerIface+".SetPosition", 0,
		dbus.ObjectPath("/org/mpris/MediaPlayer2"), pos).Err
}

// GetPosition queries the current playback position (in seconds) for the player.
func (s *Service) GetPosition(playerBusName string) (float64, error) {
	obj := s.conn.Object(playerBusName, "/org/mpris/MediaPlayer2")
	v, err := obj.GetProperty(playerIface + ".Position")
	if err != nil {
		return 0, fmt.Errorf("get position: %w", err)
	}
	if micros, ok := v.Value().(int64); ok {
		return float64(micros) / 1e6, nil
	}
	return 0, nil
}

// PlayPause toggles playback on the given player.
func (s *Service) PlayPause(playerBusName string) error {
	obj := s.conn.Object(playerBusName, "/org/mpris/MediaPlayer2")
	return obj.Call(playerIface+".PlayPause", 0).Err
}

// Next skips to the next track.
func (s *Service) Next(playerBusName string) error {
	obj := s.conn.Object(playerBusName, "/org/mpris/MediaPlayer2")
	return obj.Call(playerIface+".Next", 0).Err
}

// Previous goes to the previous track.
func (s *Service) Previous(playerBusName string) error {
	obj := s.conn.Object(playerBusName, "/org/mpris/MediaPlayer2")
	return obj.Call(playerIface+".Previous", 0).Err
}

// ActivePlayer returns the bus name of the most recently playing MPRIS player,
// or an empty string if none has been seen yet.
func (s *Service) ActivePlayer() string {
	if p := s.activePlayer.Load(); p != nil && *p != "" {
		return *p
	}
	// Fall back to the first player found on the bus.
	var names []string
	v, err := s.conn.BusObject().GetProperty("org.freedesktop.DBus.ListNames")
	if err != nil {
		return ""
	}
	if arr, ok := v.Value().([]string); ok {
		for _, name := range arr {
			if strings.HasPrefix(name, mprisPrefix) {
				names = append(names, name)
			}
		}
	}
	if len(names) > 0 {
		return names[0]
	}
	return ""
}

// PlayPauseActive toggles playback on the active player.
func (s *Service) PlayPauseActive() error {
	p := s.ActivePlayer()
	if p == "" {
		return fmt.Errorf("no active MPRIS player")
	}
	return s.PlayPause(p)
}

// NextActive skips to the next track on the active player.
func (s *Service) NextActive() error {
	p := s.ActivePlayer()
	if p == "" {
		return fmt.Errorf("no active MPRIS player")
	}
	return s.Next(p)
}

// PrevActive goes to the previous track on the active player.
func (s *Service) PrevActive() error {
	p := s.ActivePlayer()
	if p == "" {
		return fmt.Errorf("no active MPRIS player")
	}
	return s.Previous(p)
}
