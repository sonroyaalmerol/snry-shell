// Package uinput creates a virtual keyboard via /dev/uinput and sends key
// events directly to the kernel. Each keystroke is a 24-byte binary message —
// zero overhead, no daemon dependency, no process spawning.
package uinput

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	evSyn uint16 = 0
	evKey uint16 = 1

	keyEsc        uint16 = 1
	keyBackspace  uint16 = 14
	keyTab        uint16 = 15
	keyEnter      uint16 = 28
	keyLeftCtrl   uint16 = 29
	keyLeftShift  uint16 = 42
	keyRightShift uint16 = 54
	keyLeftAlt    uint16 = 56
	keySpace      uint16 = 57
	keyCapsLock   uint16 = 58
	keyLeft       uint16 = 105
	keyRight      uint16 = 106
	keyUp         uint16 = 103
	keyDown       uint16 = 108
)

// charEntry maps a rune to its evdev keycode and whether shift is needed.
type charEntry struct {
	code  uint16
	shift bool
}

// charMap maps printable characters to their evdev keycodes.
var charMap = map[rune]charEntry{
	// Letters
	'a': {30, false}, 'b': {48, false}, 'c': {46, false}, 'd': {32, false},
	'e': {18, false}, 'f': {33, false}, 'g': {34, false}, 'h': {35, false},
	'i': {23, false}, 'j': {36, false}, 'k': {37, false}, 'l': {38, false},
	'm': {50, false}, 'n': {49, false}, 'o': {24, false}, 'p': {25, false},
	'q': {16, false}, 'r': {19, false}, 's': {31, false}, 't': {20, false},
	'u': {22, false}, 'v': {47, false}, 'w': {17, false}, 'x': {45, false},
	'y': {21, false}, 'z': {44, false},
	// Numbers
	'1': {2, false}, '2': {3, false}, '3': {4, false}, '4': {5, false},
	'5': {6, false}, '6': {7, false}, '7': {8, false}, '8': {9, false},
	'9': {10, false}, '0': {11, false},
	// Symbols
	'-': {12, false}, '=': {13, false},
	'[': {26, false}, ']': {27, false},
	'\\': {43, false},
	';': {39, false}, '\'': {40, false},
	'`': {41, false},
	',': {51, false}, '.': {52, false}, '/': {53, false},
	' ': {57, false},
	// Shifted symbols
	'!': {2, true}, '@': {3, true}, '#': {4, true}, '$': {5, true},
	'%': {6, true}, '^': {7, true}, '&': {8, true}, '*': {9, true},
	'(': {10, true}, ')': {11, true},
	'_': {12, true}, '+': {13, true},
	'{': {26, true}, '}': {27, true},
	'|': {43, true},
	':': {39, true}, '"': {40, true},
	'~': {41, true},
	'<': {51, true}, '>': {52, true}, '?': {53, true},
}

// specialKeys maps OSK key names to evdev keycodes.
var specialKeys = map[string]uint16{
	"Escape":    keyEsc,
	"BackSpace": keyBackspace,
	"Tab":       keyTab,
	"Return":    keyEnter,
	"space":     keySpace,
	"Left":      keyLeft,
	"Down":      keyDown,
	"Up":        keyUp,
	"Right":     keyRight,
}

// inputEvent mirrors struct input_event from <linux/input.h>.
// struct input_event { struct timeval time; __u16 type; __u16 code; __s32 value; }
// On x86_64: timeval is 16 bytes (sec 8 + usec 8), total 24 bytes.
type inputEvent struct {
	sec  int64
	usec int64
	typ  uint16
	code uint16
	val  int32
}

// uinputSetup mirrors struct uinput_setup from <linux/uinput.h>.
type uinputSetup struct {
	name     [80]byte
	id       struct {
		bustype uint16
		vendor  uint16
		product uint16
		version uint16
	}
	ff_effects_max uint32
}

// Bridge is a virtual keyboard backed by /dev/uinput (primary) or
// ydotoold socket (fallback).
type Bridge struct {
	fd   int // /dev/uinput file descriptor
	conn *net.UnixConn
}

// New creates a virtual keyboard via /dev/uinput. If that fails (no access),
// falls back to the ydotool daemon socket. Returns nil, nil only if both fail.
func New() (*Bridge, error) {
	if b, err := newUinput(); err == nil {
		return b, nil
	}
	if b, err := newYdotoold(); err == nil {
		return b, nil
	}
	return nil, fmt.Errorf("uinput: /dev/uinput and ydotoold both unavailable")
}

// newUinput creates a virtual keyboard via /dev/uinput.
func newUinput() (*Bridge, error) {
	fd, err := unix.Open("/dev/uinput", unix.O_WRONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}

	// Enable EV_KEY and EV_SYN event types.
	if err := unix.IoctlSetInt(fd, uiSetEvBit, int(evKey)); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("uinput: UI_SET_EVBIT EV_KEY: %w", err)
	}
	if err := unix.IoctlSetInt(fd, uiSetEvBit, int(evSyn)); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("uinput: UI_SET_EVBIT EV_SYN: %w", err)
	}

	// Enable all key codes we use (KEY_ESC=1 through KEY_DOWN=111).
	for code := 0; code <= 111; code++ {
		if err := unix.IoctlSetInt(fd, uiSetKeyBit, code); err != nil {
			unix.Close(fd)
			return nil, fmt.Errorf("uinput: UI_SET_KEYBIT %d: %w", code, err)
		}
	}

	// Create the device.
	var setup uinputSetup
	copy(setup.name[:], "snry-osk-virtual\x00")
	setup.id.bustype = 0x06 // BUS_VIRTUAL
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		uiDevSetup, uintptr(unsafe.Pointer(&setup))); errno != 0 {
		unix.Close(fd)
		return nil, fmt.Errorf("uinput: UI_DEV_SETUP: %v", errno)
	}
	if err := unix.IoctlSetInt(fd, uiDevCreate, 0); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("uinput: UI_DEV_CREATE: %w", err)
	}

	return &Bridge{fd: fd}, nil
}

// newYdotoold creates a bridge to the ydotool daemon socket (fallback).
func newYdotoold() (*Bridge, error) {
	socketPath := os.Getenv("YDOTOOL_SOCKET")
	if socketPath == "" {
		socketPath = os.Getenv("XDG_RUNTIME_DIR") + "/.ydotool_socket"
	}
	addr := &net.UnixAddr{Name: socketPath, Net: "unixgram"}
	conn, err := net.DialUnix("unixgram", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("uinput: cannot connect to ydotoold: %w", err)
	}
	return &Bridge{conn: conn}, nil
}

// Close destroys the virtual keyboard or closes the daemon socket.
func (b *Bridge) Close() {
	if b.fd > 0 {
		unix.IoctlSetInt(b.fd, uiDevDestroy, 0)
		unix.Close(b.fd)
		b.fd = 0
	}
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
}

// send writes a single input_event (24 bytes) to the output.
func (b *Bridge) send(evType, code uint16, value int32) {
	if b == nil {
		return
	}
	var ev inputEvent
	ev.typ = evType
	ev.code = code
	ev.val = value

	if b.fd > 0 {
		syscall.Write(b.fd, (*[24]byte)(unsafe.Pointer(&ev))[:])
		return
	}

	if b.conn != nil {
		var buf [24]byte
		binary.LittleEndian.PutUint16(buf[16:], evType)
		binary.LittleEndian.PutUint16(buf[18:], code)
		binary.LittleEndian.PutUint32(buf[20:], uint32(value))
		b.conn.Write(buf[:])

		var syn [24]byte
		binary.LittleEndian.PutUint16(syn[16:], evSyn)
		binary.LittleEndian.PutUint16(syn[18:], 0)
		b.conn.Write(syn[:])
	}
}

// sendSyn writes a SYN_REPORT (only needed for /dev/uinput, ydotoold sends it
// automatically in send()).
func (b *Bridge) sendSyn() {
	if b != nil && b.fd > 0 {
		b.send(evSyn, 0, 0)
	}
}

// TypeChar resolves a character to its evdev keycode and sends a key press+release.
// Shift is applied automatically based on the character (e.g. "A" sends Shift+KEY_A).
// Additional modifiers (ctrl, alt) are applied if non-zero.
func (b *Bridge) TypeChar(ch string, ctrl, alt bool) {
	entry, ok := charMap[[]rune(ch)[0]]
	if !ok {
		return
	}
	var mods []uint16
	if ctrl {
		mods = append(mods, keyLeftCtrl)
	}
	if alt {
		mods = append(mods, keyLeftAlt)
	}
	if entry.shift {
		mods = append(mods, keyLeftShift)
	}
	for _, m := range mods {
		b.send(evKey, m, 1)
	}
	b.send(evKey, entry.code, 1)
	b.sendSyn()
	b.send(evKey, entry.code, 0)
	b.sendSyn()
	for i := len(mods) - 1; i >= 0; i-- {
		b.send(evKey, mods[i], 0)
		b.sendSyn()
	}
}

// TypeKey sends a special key (by name) with optional modifiers.
func (b *Bridge) TypeKey(name string, ctrl, alt, shift bool) {
	code, ok := specialKeys[name]
	if !ok {
		return
	}
	var mods []uint16
	if ctrl {
		mods = append(mods, keyLeftCtrl)
	}
	if alt {
		mods = append(mods, keyLeftAlt)
	}
	if shift {
		mods = append(mods, keyLeftShift)
	}
	for _, m := range mods {
		b.send(evKey, m, 1)
	}
	b.send(evKey, code, 1)
	b.sendSyn()
	b.send(evKey, code, 0)
	b.sendSyn()
	for i := len(mods) - 1; i >= 0; i-- {
		b.send(evKey, mods[i], 0)
		b.sendSyn()
	}
}

// Linux uinput ioctl constants. Computed from <linux/uinput.h>:
//
//	UI_SET_EVBIT  = _IOW('U', 100, int)
//	UI_SET_KEYBIT = _IOW('U', 101, int)
//	UI_DEV_SETUP  = _IOW('U', 3, struct uinput_setup)
//	UI_DEV_CREATE = _IO('U', 1)
//	UI_DEV_DESTROY = _IO('U', 2)
const (
	uiSetEvBit   = 0x40045564
	uiSetKeyBit  = 0x40045565
	uiDevSetup   = 0x405C5503
	uiDevCreate  = 0x5501
	uiDevDestroy = 0x5502
)
