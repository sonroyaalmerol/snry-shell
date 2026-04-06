// Package touchgestures provides touchscreen gesture recognition by reading
// raw evdev touch events directly from /dev/input/eventX. It uses logind's
// TakeDevice D-Bus method to obtain device access without requiring group
// membership. Supports multi-finger swipe, pinch, long press, and tap gestures.
package touchgestures

import (
	"context"
	"log"
	"math"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
)

const (
	defaultSensitivity      = 1.0
	defaultLongPressDelayMs = 400
	defaultWorkspaceFingers = 3
)

// Service reads touch events from evdev devices and recognizes gestures.
type Service struct {
	bus                   *bus.Bus
	querier               *hyprland.Querier
	sysConn               *dbus.Conn
	sensitivity           float64
	longPressDelayMs      int
	workspaceSwipeFingers int
	mu                    sync.Mutex
}

// New creates the touch gesture service. sysConn is used for logind TakeDevice
// to obtain input device access without requiring input group membership.
func New(b *bus.Bus, q *hyprland.Querier, sysConn *dbus.Conn, sensitivity float64, longPressDelayMs int, workspaceSwipeFingers int) *Service {
	if sensitivity <= 0 {
		sensitivity = defaultSensitivity
	}
	if longPressDelayMs <= 0 {
		longPressDelayMs = defaultLongPressDelayMs
	}
	if workspaceSwipeFingers <= 0 {
		workspaceSwipeFingers = defaultWorkspaceFingers
	}
	return &Service{
		bus:                   b,
		querier:               q,
		sysConn:               sysConn,
		sensitivity:           sensitivity,
		longPressDelayMs:      longPressDelayMs,
		workspaceSwipeFingers: workspaceSwipeFingers,
	}
}

// Run starts reading touch events and recognizing gestures.
// Retries periodically if no touch device is found.
// Blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		devices, err := findTouchDevices()
		if err != nil {
			log.Printf("[GESTURES] find devices: %v", err)
		}

		if len(devices) > 0 {
			log.Printf("[GESTURES] found %d touch device(s): %v", len(devices), devices)
			for _, devPath := range devices {
				dev, err := openTouchDevice(devPath, s.sysConn)
				if err != nil {
					log.Printf("[GESTURES] open %s: %v", devPath, err)
					continue
				}
				log.Printf("[GESTURES] opened %s (%s): slots=%d x=[%d,%d] y=[%d,%d]",
					devPath, dev.Name, dev.MaxSlots, dev.XRange[0], dev.XRange[1], dev.YRange[0], dev.YRange[1])
				go s.readDevice(ctx, dev)
			}
			break
		}

		log.Printf("[GESTURES] no touch devices found, retrying in 5s")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}

	<-ctx.Done()
	return ctx.Err()
}

// readDevice reads evdev events from a single touch device and processes them.
func (s *Service) readDevice(ctx context.Context, dev *TouchDevice) {
	rawCh := make(chan evRawEvent, 64)
	go readEvents(ctx, dev, rawCh)

	engine := NewGestureEngine(s.sensitivity, s.longPressDelayMs, func(result GestureResult) {
		s.dispatch(result)
	})

	// Slot-based tracking state.
	slots := make([]TouchPoint, dev.MaxSlots)
	currentSlot := 0

	for ev := range rawCh {
		select {
		case <-ctx.Done():
			return
		default:
		}

		switch ev.typ {
		case evAbs:
			switch ev.code {
			case absMTSlot:
				if ev.val >= 0 && ev.val < int32(dev.MaxSlots) {
					currentSlot = int(ev.val)
				}
			case absMTTrackingID:
				slots[currentSlot].Slot = currentSlot
				slots[currentSlot].TrackingID = int(ev.val)
				slots[currentSlot].Active = ev.val != -1
				if ev.val == -1 {
					slots[currentSlot].X = 0
					slots[currentSlot].Y = 0
				}
			case absMTPositionX:
				slots[currentSlot].X = s.normalize(ev.val, dev.XRange)
			case absMTPositionY:
				slots[currentSlot].Y = s.normalize(ev.val, dev.YRange)
			}

		case evSyn:
			if ev.code == synReport {
				frame := buildFrame(slots)
				engine.Feed(frame, time.Now().UnixNano())
			} else if ev.code == synDropped {
				engine.Reset()
			}
		}
	}
}

// normalize converts a raw axis value to a 0-1 range for consistent
// gesture math regardless of screen resolution.
func (s *Service) normalize(val int32, range_ [2]int32) float64 {
	min, max := float64(range_[0]), float64(range_[1])
	if max == min {
		return 0.5
	}
	normalized := (float64(val) - min) / (max - min)
	return math.Max(0, math.Min(1, normalized))
}

// dispatch maps a recognized gesture to an action.
func (s *Service) dispatch(r GestureResult) {
	log.Printf("[GESTURES] %s dir=%d fingers=%d scale=%.2f",
		gestureTypeString(r.Type), r.Direction, r.Fingers, r.Scale)

	s.bus.Publish(bus.TopicGesture, r)

	s.mu.Lock()
	defer s.mu.Unlock()

	switch r.Type {
	case GestureSwipe:
		if r.Fingers == s.workspaceSwipeFingers {
			switch {
			case r.Direction == DirLeft:
				if s.querier != nil {
					go s.querier.SwitchWorkspace(-1)
				}
			case r.Direction == DirRight:
				if s.querier != nil {
					go s.querier.SwitchWorkspace(1)
				}
			case r.Direction == DirUp:
				s.bus.Publish(bus.TopicSystemControls, "toggle-overview")
			case r.Direction == DirDown:
				s.bus.Publish(bus.TopicSystemControls, "toggle-notifcenter")
			}
		}
	}
}

func gestureTypeString(t GestureType) string {
	switch t {
	case GestureSwipe:
		return "swipe"
	case GesturePinch:
		return "pinch"
	case GestureLongPress:
		return "longpress"
	case GestureTap:
		return "tap"
	default:
		return "unknown"
	}
}

// buildFrame extracts active touch points from slot state.
func buildFrame(slots []TouchPoint) touchFrame {
	frame := touchFrame{fingers: make(map[int]PointF)}
	for _, tp := range slots {
		if tp.Active && tp.TrackingID >= 0 {
			frame.fingers[tp.Slot] = PointF{X: tp.X, Y: tp.Y}
			frame.count++
		}
	}
	return frame
}
