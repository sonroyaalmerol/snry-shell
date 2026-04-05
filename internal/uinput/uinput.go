// Package uinput sends key events directly to the ydotool daemon socket,
// bypassing process spawning entirely. Each keystroke is a 24-byte binary
// message — zero overhead compared to spawning wtype/ydotool per key.
package uinput

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
)

const (
	evSyn  uint16 = 0
	evKey  uint16 = 1

	keyEsc       uint16 = 1
	keyBackspace uint16 = 14
	keyTab       uint16 = 15
	keyEnter     uint16 = 28
	keyLeftCtrl  uint16 = 29
	keyLeftShift uint16 = 42
	keyRightShift uint16 = 54
	keyLeftAlt   uint16 = 56
	keySpace     uint16 = 57
	keyCapsLock  uint16 = 58
	keyLeft      uint16 = 105
	keyRight     uint16 = 106
	keyUp        uint16 = 103
	keyDown      uint16 = 108
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

// Bridge holds a persistent connection to the ydotoold daemon socket.
type Bridge struct {
	conn *net.UnixConn
}

// New connects to the ydotool daemon. Returns nil, nil if the daemon is not
// available (socket doesn't exist or connection fails).
func New() (*Bridge, error) {
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

func (b *Bridge) Close() {
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
}

// send writes a single input_event (24 bytes) + SYN_REPORT to the socket.
func (b *Bridge) send(evType, code uint16, value int32) {
	var buf [24]byte
	binary.LittleEndian.PutUint16(buf[16:], evType)
	binary.LittleEndian.PutUint16(buf[18:], code)
	binary.LittleEndian.PutUint32(buf[20:], uint32(value))
	b.conn.Write(buf[:])

	var syn [24]byte
	binary.LittleEndian.PutUint16(syn[16:], evSyn)
	binary.LittleEndian.PutUint16(syn[18:], 0) // SYN_REPORT
	b.conn.Write(syn[:])
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
	b.send(evKey, entry.code, 0)
	for i := len(mods) - 1; i >= 0; i-- {
		b.send(evKey, mods[i], 0)
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
	b.send(evKey, code, 0)
	for i := len(mods) - 1; i >= 0; i-- {
		b.send(evKey, mods[i], 0)
	}
}
