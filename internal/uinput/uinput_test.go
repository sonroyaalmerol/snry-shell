package uinput

import (
	"testing"
	"unsafe"
)

func TestInputEventSize(t *testing.T) {
	// struct input_event must be exactly 24 bytes on x86_64:
	//   struct timeval { time_t sec (8); suseconds_t usec (8) } = 16
	//   __u16 type (2) + __u16 code (2) + __s32 value (4) = 8
	//   total = 24
	if sz := unsafe.Sizeof(inputEvent{}); sz != 24 {
		t.Fatalf("inputEvent size = %d, want 24", sz)
	}
}

func TestInputEventLayout(t *testing.T) {
	// Verify type/code/value are at the correct offsets (after timeval).
	var ev inputEvent
	ev.typ = 1
	ev.code = 30
	ev.val = -1

	buf := (*[24]byte)(unsafe.Pointer(&ev))

	// Bytes 0-15: timeval (zeroed).
	for i := 0; i < 16; i++ {
		if buf[i] != 0 {
			t.Fatalf("timeval byte %d non-zero: %d", i, buf[i])
		}
	}
	// Bytes 16-17: type = 1 (little-endian uint16).
	if buf[16] != 1 || buf[17] != 0 {
		t.Fatalf("type offset wrong: got %d %d, want 1 0", buf[16], buf[17])
	}
	// Bytes 18-19: code = 30 (little-endian uint16).
	if buf[18] != 30 || buf[19] != 0 {
		t.Fatalf("code offset wrong: got %d %d, want 30 0", buf[18], buf[19])
	}
	// Bytes 20-23: value = -1 (little-endian int32).
	if buf[20] != 0xFF || buf[21] != 0xFF || buf[22] != 0xFF || buf[23] != 0xFF {
		t.Fatalf("value offset wrong: got %x %x %x %x", buf[20], buf[21], buf[22], buf[23])
	}
}

func TestUinputSetupSize(t *testing.T) {
	// struct uinput_setup: char name[80] + struct input_id (8) + __u32 = 92
	if sz := unsafe.Sizeof(uinputSetup{}); sz != 92 {
		t.Fatalf("uinputSetup size = %d, want 92", sz)
	}
}

func TestCharMapLowercase(t *testing.T) {
	tests := []struct {
		ch   rune
		code uint16
	}{
		{'a', 30}, {'b', 48}, {'z', 44},
		{'0', 11}, {'1', 2}, {'9', 10},
		{' ', 57}, {'\n', 0}, // newline not in map
	}
	for _, tt := range tests {
		e, ok := charMap[tt.ch]
		if tt.ch == '\n' {
			if ok {
				t.Errorf("newline should not be in charMap")
			}
			continue
		}
		if !ok {
			t.Errorf("rune %q not found in charMap", tt.ch)
			continue
		}
		if e.code != tt.code {
			t.Errorf("rune %q: code=%d, want %d", tt.ch, e.code, tt.code)
		}
		if e.shift {
			t.Errorf("rune %q: shift=true, want false", tt.ch)
		}
	}
}

func TestCharMapShifted(t *testing.T) {
	tests := []struct {
		ch    rune
		code  uint16
		shift bool
	}{
		// Shifted symbols share keycodes with their base key.
		{'!', 2, true},
		{'@', 3, true},
		{'?', 53, true},
		{':', 39, true},
		{'"', 40, true},
	}
	for _, tt := range tests {
		e, ok := charMap[tt.ch]
		if !ok {
			t.Errorf("rune %q not found in charMap", tt.ch)
			continue
		}
		if e.code != tt.code {
			t.Errorf("rune %q: code=%d, want %d", tt.ch, e.code, tt.code)
		}
		if e.shift != tt.shift {
			t.Errorf("rune %q: shift=%v, want %v", tt.ch, e.shift, tt.shift)
		}
	}
	// Uppercase letters are not in charMap — the OSK resolves them via
	// the lowercase entry + shift modifier.
	if _, ok := charMap['A']; ok {
		t.Error("uppercase 'A' should not be in charMap (use 'a' + shift)")
	}
}

func TestSpecialKeys(t *testing.T) {
	tests := []struct {
		name string
		code uint16
	}{
		{"Escape", 1}, {"BackSpace", 14}, {"Tab", 15},
		{"Return", 28}, {"Left", 105}, {"Right", 106},
		{"Up", 103}, {"Down", 108}, {"space", 57},
	}
	for _, tt := range tests {
		got, ok := specialKeys[tt.name]
		if !ok {
			t.Errorf("special key %q not found", tt.name)
			continue
		}
		if got != tt.code {
			t.Errorf("special key %q: code=%d, want %d", tt.name, got, tt.code)
		}
	}
}

func TestTypeCharNilBridge(t *testing.T) {
	// Calling methods on nil Bridge must not panic.
	var b *Bridge = nil
	b.TypeChar("a", false, false)
	b.TypeKey("Escape", false, false, false)
}

func TestNewUinput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping /dev/uinput test in short mode")
	}
	b, err := newUinput()
	if err != nil {
		t.Skipf("/dev/uinput not available: %v", err)
	}
	defer b.Close()

	if b.fd <= 0 {
		t.Fatal("expected valid fd, got 0")
	}

	// Send a key event — should not error or panic.
	b.TypeChar("a", false, false)
	b.TypeChar("A", false, false)
	b.TypeKey("Escape", false, false, false)
	b.TypeKey("BackSpace", true, false, false)
}

func TestIoctlConstants(t *testing.T) {
	// Verify ioctl constants match what _IOW/_IO would produce on this arch.
	// _IOW(type, nr, size) = (1 << 30) | ((size & 0x3FFF) << 16) | (type << 8) | nr
	// _IO(type, nr) = (type << 8) | nr

	iow := func(typ, nr, size int) uint32 {
		return uint32((1 << 30) | ((size & 0x3FFF) << 16) | (typ << 8) | nr)
	}
	io := func(typ, nr int) uint32 {
		return uint32((typ << 8) | nr)
	}

	const (
		U = int('U')
	)
	const (
		sizeofInt         = 4
		sizeofUinputSetup = 92
	)

	tests := []struct {
		name   string
		got    uint32
		expect uint32
	}{
		{"UI_SET_EVBIT", uiSetEvBit, iow(U, 100, sizeofInt)},
		{"UI_SET_KEYBIT", uiSetKeyBit, iow(U, 101, sizeofInt)},
		{"UI_DEV_SETUP", uiDevSetup, iow(U, 3, sizeofUinputSetup)},
		{"UI_DEV_CREATE", uiDevCreate, io(U, 1)},
		{"UI_DEV_DESTROY", uiDevDestroy, io(U, 2)},
	}

	for _, tt := range tests {
		if tt.got != tt.expect {
			t.Errorf("%s = 0x%X, want 0x%X", tt.name, tt.got, tt.expect)
		}
	}
}
