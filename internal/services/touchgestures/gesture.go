package touchgestures

import (
	"math"
	"time"
)

// Direction bitmask constants.
const (
	DirNone   uint32 = 0
	DirLeft   uint32 = 1 << iota
	DirRight
	DirUp
	DirDown
)

// GestureType identifies the kind of gesture detected.
type GestureType int

const (
	GestureSwipe    GestureType = iota // Multi-finger swipe
	GesturePinch                       // Pinch in/out
	GestureLongPress                   // Hold still
	GestureTap                         // Quick tap
)

// GestureResult is emitted when a gesture is recognized.
type GestureResult struct {
	Type      GestureType
	Direction uint32
	Fingers   int
	Scale     float64 // pinch only: <1 = pinch in, >1 = pinch out
}

// PointF is a 2D point used for gesture math.
type PointF struct {
	X, Y float64
}

func (p PointF) delta(o PointF) PointF {
	return PointF{X: p.X - o.X, Y: p.Y - o.Y}
}

func (p PointF) length() float64 {
	return math.Sqrt(p.X*p.X + p.Y*p.Y)
}

// gestureState tracks all active fingers for a single gesture.
type gestureState struct {
	fingers map[int]*fingerState // slot → finger
	started bool
}

type fingerState struct {
	origin  PointF
	current PointF
	active  bool
}

// touchPoint is a resolved touch frame: one entry per active finger.
type touchFrame struct {
	fingers map[int]PointF // slot → current position
	count   int
}

// GestureEngine recognizes gestures from a stream of touch frames.
type GestureEngine struct {
	state      *gestureState
	sensitivity float64
	swipeThreshold   float64
	swipeSlipTol     float64
	pinchThreshold   float64
	longPressDelayMs int

	// Swipe tracking.
	swipeFingers   int
	swipeDir       uint32
	swipeLocked    bool
	swipeCompleted bool

	// Pinch tracking.
	pinchActive   bool
	pinchInitial  float64
	pinchCompleted bool

	// Long press tracking.
	longPressActive  bool
	longPressOrigin  PointF
	longPressCenter  PointF
	longPressTimer   *longPressTimer

	// Tap tracking.
	tapStartTime int64 // monotonic nanoseconds

	onGesture func(GestureResult)
}

type longPressTimer struct {
	timer *time.Timer
}

func newLongPressTimer(delayMs int, callback func()) *longPressTimer {
	return &longPressTimer{
		timer: time.AfterFunc(time.Duration(delayMs)*time.Millisecond, callback),
	}
}

func (lpt *longPressTimer) cancel() {
	if lpt.timer != nil {
		lpt.timer.Stop()
	}
}

// NewGestureEngine creates a gesture recognition engine.
func NewGestureEngine(sensitivity float64, longPressDelayMs int, onGesture func(GestureResult)) *GestureEngine {
	ge := &GestureEngine{
		sensitivity:      sensitivity,
		swipeThreshold:   150 / sensitivity,
		swipeSlipTol:     100 / sensitivity,
		pinchThreshold:   150 / sensitivity,
		longPressDelayMs: longPressDelayMs,
		onGesture:        onGesture,
	}
	return ge
}

// Reset clears all gesture state.
func (ge *GestureEngine) Reset() {
	if ge.longPressTimer != nil {
		ge.longPressTimer.cancel()
		ge.longPressTimer = nil
	}
	ge.state = nil
	ge.swipeFingers = 0
	ge.swipeDir = 0
	ge.swipeLocked = false
	ge.swipeCompleted = false
	ge.pinchActive = false
	ge.pinchInitial = 0
	ge.pinchCompleted = false
	ge.longPressActive = false
}

// Feed processes a touch frame. Call on every SYN_REPORT.
func (ge *GestureEngine) Feed(frame touchFrame, nowNs int64) {
	n := frame.count

	// No fingers — all lifted.
	if n == 0 {
		ge.handleLift()
		ge.Reset()
		return
	}

	// First touch — initialize state.
	if ge.state == nil || !ge.state.started {
		ge.state = &gestureState{fingers: make(map[int]*fingerState, n)}
		ge.state.started = true
		for slot, pos := range frame.fingers {
			ge.state.fingers[slot] = &fingerState{origin: pos, current: pos, active: true}
		}
		ge.swipeFingers = n
		ge.tapStartTime = nowNs
		ge.startLongPress(frame)
		return
	}

	// Update finger positions.
	for slot, pos := range frame.fingers {
		f, ok := ge.state.fingers[slot]
		if ok {
			f.current = pos
		} else {
			ge.state.fingers[slot] = &fingerState{origin: pos, current: pos, active: true}
		}
	}

	// Check long press movement.
	if ge.longPressActive {
		center := ge.getCenter()
		moved := center.delta(ge.longPressCenter).length()
		if moved > ge.swipeSlipTol {
			// Moved too much for long press — cancel it.
			ge.cancelLongPress()
		}
	}

	// Run gesture updates.
	if n >= 2 {
		ge.updatePinch(frame)
	}
	if n >= 2 {
		ge.updateSwipe(frame)
	}
}

// handleLift is called when all fingers are lifted.
func (ge *GestureEngine) handleLift() {
	// Check for tap (single finger, short duration, minimal movement).
	if ge.state != nil && ge.swipeFingers == 1 && !ge.swipeCompleted && !ge.pinchCompleted {
		if ge.longPressTimer != nil {
			ge.longPressTimer.cancel()
			ge.longPressTimer = nil
		}
		center := ge.getCenter()
		moved := center.delta(ge.getOriginCenter()).length()
		if moved < ge.swipeSlipTol {
			ge.onGesture(GestureResult{Type: GestureTap, Fingers: 1})
			return
		}
	}

	// Emit completed swipe.
	if ge.swipeLocked && !ge.swipeCompleted {
		ge.swipeCompleted = true
		ge.onGesture(GestureResult{
			Type:      GestureSwipe,
			Direction: ge.swipeDir,
			Fingers:   ge.swipeFingers,
		})
	}

	// Emit completed pinch.
	if ge.pinchActive && !ge.pinchCompleted {
		ge.pinchCompleted = true
		scale := ge.getPinchScale()
		dir := DirUp // pinch in (fingers closer)
		if scale >= 1.0 {
			dir = DirDown // pinch out (fingers apart)
		}
		ge.onGesture(GestureResult{
			Type:      GesturePinch,
			Direction: dir,
			Fingers:   ge.swipeFingers,
			Scale:     scale,
		})
	}
}

// updateSwipe checks for multi-finger swipe gestures.
func (ge *GestureEngine) updateSwipe(frame touchFrame) {
	if ge.swipeCompleted {
		return
	}

	center := ge.getCenter()
	origin := ge.getOriginCenter()
	delta := center.delta(origin)
	dist := delta.length()

	if dist < ge.swipeThreshold {
		return
	}

	if !ge.swipeLocked {
		dir := getDirection(delta)
		if dir == DirNone {
			return
		}
		ge.swipeDir = dir
		ge.swipeLocked = true
		// Cancel long press once swipe is recognized.
		ge.cancelLongPress()
	}

	// Check finger slip tolerance.
	for _, f := range ge.state.fingers {
		if !f.active {
			continue
		}
		fDelta := f.current.delta(f.origin)
		incorrect := getIncorrectDragDistance(fDelta, ge.swipeDir)
		if incorrect > ge.swipeSlipTol {
			// Too much slip — cancel swipe.
			ge.swipeLocked = false
			return
		}
	}

	// Check if swipe distance in target direction meets threshold.
	dragDist := getDragDistance(delta, ge.swipeDir)
	if dragDist >= ge.swipeThreshold {
		ge.swipeCompleted = true
		ge.cancelLongPress()
		ge.onGesture(GestureResult{
			Type:      GestureSwipe,
			Direction: ge.swipeDir,
			Fingers:   ge.swipeFingers,
		})
	}
}

// updatePinch checks for pinch gestures (2+ fingers).
func (ge *GestureEngine) updatePinch(frame touchFrame) {
	if ge.pinchCompleted || frame.count < 2 {
		return
	}

	span := ge.getSpan()

	if !ge.pinchActive {
		ge.pinchInitial = span
		ge.pinchActive = true
		return
	}

	if math.Abs(span-ge.pinchInitial) > ge.pinchThreshold {
		ge.pinchCompleted = true
		ge.cancelLongPress()
		scale := span / ge.pinchInitial
		dir := DirUp // pinch in
		if scale >= 1.0 {
			dir = DirDown // pinch out
		}
		ge.onGesture(GestureResult{
			Type:      GesturePinch,
			Direction: dir,
			Fingers:   frame.count,
			Scale:     scale,
		})
	}
}

// startLongPress begins the long press timer.
func (ge *GestureEngine) startLongPress(frame touchFrame) {
	center := ge.getCenter()
	ge.longPressActive = true
	ge.longPressOrigin = center
	ge.longPressCenter = center
	ge.longPressTimer = newLongPressTimer(ge.longPressDelayMs, func() {
		ge.longPressActive = false
		ge.onGesture(GestureResult{
			Type:    GestureLongPress,
			Fingers: len(frame.fingers),
		})
	})
}

func (ge *GestureEngine) cancelLongPress() {
	if ge.longPressTimer != nil {
		ge.longPressTimer.cancel()
		ge.longPressTimer = nil
	}
	ge.longPressActive = false
}

// getCenter returns the centroid of all active fingers.
func (ge *GestureEngine) getCenter() PointF {
	if ge.state == nil || len(ge.state.fingers) == 0 {
		return PointF{}
	}
	var cx, cy float64
	n := 0
	for _, f := range ge.state.fingers {
		cx += f.current.X
		cy += f.current.Y
		n++
	}
	return PointF{X: cx / float64(n), Y: cy / float64(n)}
}

// getOriginCenter returns the centroid of all finger origins.
func (ge *GestureEngine) getOriginCenter() PointF {
	if ge.state == nil || len(ge.state.fingers) == 0 {
		return PointF{}
	}
	var cx, cy float64
	n := 0
	for _, f := range ge.state.fingers {
		cx += f.origin.X
		cy += f.origin.Y
		n++
	}
	return PointF{X: cx / float64(n), Y: cy / float64(n)}
}

// getSpan computes the AOSP-style span: 2 * average distance from centroid.
func (ge *GestureEngine) getSpan() float64 {
	if ge.state == nil || len(ge.state.fingers) < 2 {
		return 0
	}
	center := ge.getCenter()
	var total float64
	n := 0
	for _, f := range ge.state.fingers {
		d := f.current.delta(center).length()
		total += d
		n++
	}
	return 2 * total / float64(n)
}

// getPinchScale returns current span / initial span.
func (ge *GestureEngine) getPinchScale() float64 {
	if ge.pinchInitial == 0 {
		return 1.0
	}
	return ge.getSpan() / ge.pinchInitial
}

// getDirection determines the direction bitmask from a delta vector.
// Uses a tangent threshold of 1/3 to allow diagonal detection.
func getDirection(delta PointF) uint32 {
	threshold := 1.0 / 3.0
	var dir uint32

	if math.Abs(delta.X) > threshold*math.Abs(delta.Y) {
		dir |= DirLeft | DirRight
	}
	if math.Abs(delta.Y) > threshold*math.Abs(delta.X) {
		dir |= DirUp | DirDown
	}

	if dir&DirLeft != 0 && dir&DirRight != 0 {
		if delta.X > 0 {
			dir &= ^DirLeft
		} else {
			dir &= ^DirRight
		}
	}
	if dir&DirUp != 0 && dir&DirDown != 0 {
		if delta.Y > 0 {
			dir &= ^DirUp
		} else {
			dir &= ^DirDown
		}
	}

	return dir
}

// getIncorrectDragDistance returns the component of movement perpendicular
// to (or opposite to) the target direction, using Graham-Schmidt decomposition.
func getIncorrectDragDistance(delta PointF, direction uint32) float64 {
	var dx, dy float64
	switch {
	case direction == DirRight:
		dx, dy = math.Abs(delta.X), math.Abs(delta.Y)
	case direction == DirLeft:
		dx, dy = math.Abs(delta.X), math.Abs(delta.Y)
	case direction == DirUp:
		dx, dy = math.Abs(delta.Y), math.Abs(delta.X)
	case direction == DirDown:
		dx, dy = math.Abs(delta.Y), math.Abs(delta.X)
	case direction&(DirLeft|DirRight) != 0:
		dx, dy = math.Abs(delta.X), math.Abs(delta.Y)
	case direction&(DirUp|DirDown) != 0:
		dx, dy = math.Abs(delta.Y), math.Abs(delta.X)
	default:
		return delta.length()
	}

	// Graham-Schmidt: incorrect = dy - (dy/dx)*dx when dx > dy component.
	if dx < dy {
		return dy
	}
	if dx == 0 {
		return dy
	}
	return math.Abs(dy - (dy/dx)*dx)
}

// getDragDistance returns the movement distance in the target direction.
func getDragDistance(delta PointF, direction uint32) float64 {
	switch {
	case direction == DirRight:
		return delta.X
	case direction == DirLeft:
		return -delta.X
	case direction == DirUp:
		return -delta.Y
	case direction == DirDown:
		return delta.Y
	default:
		return delta.length()
	}
}
