// Package touchgestures provides touchscreen gesture recognition by reading
// raw evdev touch events directly from /dev/input/eventX. It supports
// multi-finger swipe, pinch, long press, and tap gestures, dispatching
// actions via the bus and Hyprland IPC.
package touchgestures

import (
	"context"
	"log"
	"math"
	"sync"
	"time"

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
	sensitivity           float64
	longPressDelayMs      int
	workspaceSwipeFingers int
	mu                    sync.Mutex
}

// New creates the touch gesture service.
func New(b *bus.Bus, q *hyprland.Querier, sensitivity float64, longPressDelayMs int, workspaceSwipeFingers int) *Service {
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
		sensitivity:           sensitivity,
		longPressDelayMs:      longPressDelayMs,
		workspaceSwipeFingers: workspaceSwipeFingers,
	}
}

// Run starts reading touch events and recognizing gestures.
// Blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	devices, err := findTouchDevices()
	if err != nil {
		log.Printf("[GESTURES] no touch devices found: %v", err)
		return nil
	}
	if len(devices) == 0 {
		log.Printf("[GESTURES] no touch devices found")
		return nil
	}
	log.Printf("[GESTURES] found %d touch device(s): %v", len(devices), devices)

	for _, devPath := range devices {
		dev, err := openTouchDevice(devPath)
		if err != nil {
			log.Printf("[GESTURES] open %s: %v", devPath, err)
			continue
		}
		log.Printf("[GESTURES] opened %s (%s): slots=%d x=[%d,%d] y=[%d,%d]",
			devPath, dev.Name, dev.MaxSlots, dev.XRange[0], dev.XRange[1], dev.YRange[0], dev.YRange[1])

		go s.readDevice(ctx, dev)
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
				// Events were dropped — reset state.
				engine.Reset()
			}

		case evKey:
			// BTN_TOUCH can be used for legacy single-touch detection.
			// We primarily use the MT protocol above.
			_ = ev.val
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
	// Clamp to [0, 1].
	return math.Max(0, math.Min(1, normalized))
}

// dispatch maps a recognized gesture to an action.
func (s *Service) dispatch(r GestureResult) {
	log.Printf("[GESTURES] %s dir=%d fingers=%d scale=%.2f",
		gestureTypeString(r.Type), r.Direction, r.Fingers, r.Scale)

	// Always publish on the bus for any subscriber.
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

	case GesturePinch:
		// Pinch gestures are published on the bus for future use.
		// Potential actions: toggle overview, change workspace scale, etc.

	case GestureLongPress:
		// Long press is published on the bus for future use.
		// Potential actions: context menu, drag-to-move mode, etc.

	case GestureTap:
		// Single-finger tap is published for future use.
		// Potential actions: click, select, etc.
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
