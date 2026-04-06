package touchgestures

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strings"
	"unsafe"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"
)

// evdev event types.
const (
	evSyn = 0x00
	evKey = 0x01
	evAbs = 0x03
)

// evdev sync codes.
const (
	synReport  = 0
	synDropped = 3
)

// evdev absolute axis codes.
const (
	absX            = 0x00
	absY            = 0x01
	absMTSlot       = 0x2f
	absMTTrackingID = 0x39
	absMTPositionX  = 0x35
	absMTPositionY  = 0x36
	absMTPressure   = 0x3a
)

// input device properties.
const (
	inputPropDirect  = 0x01
	inputPropPointer = 0x02
)

// logind constants.
const (
	logindDest  = "org.freedesktop.login1"
	logindIface = "org.freedesktop.login1.Session"
)

// TouchPoint represents a single tracked finger.
type TouchPoint struct {
	Slot       int
	TrackingID int
	X, Y       float64
	Active     bool
}

// TouchDevice describes an opened multitouch input device.
type TouchDevice struct {
	Path     string
	Name     string
	File     *os.File
	MaxSlots int
	XRange   [2]int32
	YRange   [2]int32
}

// inputAbsInfo mirrors struct input_absinfo from <linux/input.h>.
type inputAbsInfo struct {
	Value      int32
	Minimum    int32
	Maximum    int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}

// findTouchDevices identifies touchscreens by reading sysfs capabilities
// and /proc/bus/input/devices. Does not open any device files, so it works
// without special permissions.
func findTouchDevices() ([]string, error) {
	data, err := os.ReadFile("/proc/bus/input/devices")
	if err != nil {
		return nil, fmt.Errorf("read /proc/bus/input/devices: %w", err)
	}

	var devices []string
	var handlers []string
	var name string

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// End of device block — check capabilities via sysfs.
			for _, h := range handlers {
				if isTouchDevice(h, name) {
					devices = append(devices, "/dev/input/"+h)
				}
			}
			handlers = nil
			name = ""
			continue
		}
		if strings.HasPrefix(line, "N: Name=") {
			name = strings.TrimPrefix(line, "N: Name=\"")
			name = strings.TrimSuffix(name, "\"")
		}
		if strings.HasPrefix(line, "H: Handlers=") {
			for _, field := range strings.Split(line, " ") {
				if strings.HasPrefix(field, "event") {
					handlers = append(handlers, field)
				}
			}
		}
	}
	return devices, scanner.Err()
}

// isTouchDevice checks sysfs for multitouch capability and that the device
// is not a pointer/stylus/touchpad.
func isTouchDevice(eventName, devName string) bool {
	// Check ABS capabilities via sysfs.
	absCaps, err := os.ReadFile(fmt.Sprintf("/sys/class/input/%s/device/capabilities/abs", eventName))
	if err != nil {
		return false
	}
	// Check for ABS_MT_POSITION_X (bit 53) in the hex bitmask.
	hasMT := hasBit(string(absCaps), absMTPositionX)
	if !hasMT {
		return false
	}

	// Exclude pointer devices (stylus, mouse, touchpad).
	propCaps, err := os.ReadFile(fmt.Sprintf("/sys/class/input/%s/device/properties", eventName))
	if err == nil {
		hasPropPointer := hasBit(string(propCaps), inputPropPointer)
		if hasPropPointer {
			return false
		}
	}

	return true
}

// hasBit checks if bit n is set in a hex bitmask string like "3 800000000".
func hasBit(hexStr string, n int) bool {
	hexStr = strings.TrimSpace(hexStr)
	// sysfs bitmask format: space-separated groups of hex digits, MSB first.
	// Group 0 is bits 0-31, group 1 is bits 32-63, etc.
	group := n / 32
	bit := uint(n % 32)

	fields := strings.Fields(hexStr)
	if group >= len(fields) {
		return false
	}

	var val uint32
	fmt.Sscanf(fields[len(fields)-1-group], "%x", &val)
	return val&(1<<bit) != 0
}

// openTouchDevice opens a touch device using logind's TakeDevice to obtain
// an fd without requiring direct file permissions. Falls back to direct open.
func openTouchDevice(path string, sysConn *dbus.Conn) (*TouchDevice, error) {
	var fd int

	// Try logind TakeDevice first (works without group membership).
	if sysConn != nil {
		taken, err := takeDevice(sysConn, path)
		if err != nil {
			log.Printf("[GESTURES] logind TakeDevice %s: %v, trying direct open", path, err)
		} else {
			fd = taken
		}
	}

	// Fall back to direct open.
	if fd <= 0 {
		rawFd, err := unix.Open(path, unix.O_RDONLY, 0)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w (logind and direct both failed)", path, err)
		}
		fd = int(rawFd)
	}

	// Verify MT capability.
	evBits := make([]byte, 128)
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL, uintptr(fd),
		uintptr(evIOCGBIT(evAbs, len(evBits))),
		uintptr(unsafe.Pointer(&evBits[0])),
	)
	if errno != 0 {
		unix.Close(fd)
		return nil, fmt.Errorf("ioctl EVIOCGBIT EV_ABS: %w", errno)
	}
	if evBits[absMTPositionX/8]&(1<<(absMTPositionX%8)) == 0 {
		unix.Close(fd)
		return nil, fmt.Errorf("%s: no multitouch support", path)
	}

	// Get max slots.
	var slotInfo inputAbsInfo
	_, _, errno = unix.Syscall(
		unix.SYS_IOCTL, uintptr(fd),
		uintptr(evIOCGABS(absMTSlot)),
		uintptr(unsafe.Pointer(&slotInfo)),
	)
	maxSlots := int(slotInfo.Maximum) + 1
	if errno != 0 || maxSlots <= 0 {
		maxSlots = 10
	}

	// Get coordinate ranges.
	var xInfo inputAbsInfo
	unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(evIOCGABS(absMTPositionX)), uintptr(unsafe.Pointer(&xInfo)))
	var yInfo inputAbsInfo
	unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(evIOCGABS(absMTPositionY)), uintptr(unsafe.Pointer(&yInfo)))

	// Read device name.
	var devName [256]byte
	_, _, _ = unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(evIOCGNAME(256)), uintptr(unsafe.Pointer(&devName[0])))
	name := strings.TrimRight(string(devName[:]), "\x00")

	return &TouchDevice{
		Path:     path,
		Name:     name,
		File:     os.NewFile(uintptr(fd), path),
		MaxSlots: maxSlots,
		XRange:   [2]int32{xInfo.Minimum, xInfo.Maximum},
		YRange:   [2]int32{yInfo.Minimum, yInfo.Maximum},
	}, nil
}

// takeDevice uses logind's TakeDevice to get a file descriptor for an input device.
// This works because the shell runs under the same session as the compositor.
func takeDevice(conn *dbus.Conn, devPath string) (int, error) {
	// Get device major:minor from stat.
	var stat unix.Stat_t
	if err := unix.Stat(devPath, &stat); err != nil {
		return 0, err
	}
	major := unix.Major(uint64(stat.Rdev))
	minor := unix.Minor(uint64(stat.Rdev))

	// Resolve session path.
	sessionPath, err := resolveLogindSession(conn)
	if err != nil {
		return 0, fmt.Errorf("resolve session: %w", err)
	}

	obj := conn.Object(logindDest, dbus.ObjectPath(sessionPath))
	var fd dbus.UnixFD
	err = obj.Call(logindIface+".TakeDevice", 0, uint32(major), uint32(minor)).Store(&fd, nil)
	if err != nil {
		return 0, err
	}
	return int(fd), nil
}

func resolveLogindSession(conn *dbus.Conn) (string, error) {
	if id := os.Getenv("XDG_SESSION_ID"); id != "" {
		return "/org/freedesktop/login1/session_" + id, nil
	}
	mgr := conn.Object(logindDest, "/org/freedesktop/login1")
	var sessionPath dbus.ObjectPath
	if err := mgr.Call("org.freedesktop.login1.Manager.GetSessionByPID", 0, uint32(os.Getpid())).Store(&sessionPath); err != nil {
		return "", err
	}
	return string(sessionPath), nil
}

// readEvents reads raw evdev events from the device file and sends parsed
// events on the returned channel. Blocks until ctx is cancelled.
func readEvents(ctx context.Context, dev *TouchDevice, ch chan<- evRawEvent) {
	defer dev.File.Close()
	defer close(ch)

	buf := make([]byte, 24) // sizeof(struct input_event) on x86_64
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := dev.File.Read(buf)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				return
			}
			if n != 24 {
				continue
			}
			typ := binary.LittleEndian.Uint16(buf[16:18])
			code := binary.LittleEndian.Uint16(buf[18:20])
			val := int32(binary.LittleEndian.Uint32(buf[20:24]))
			ch <- evRawEvent{typ: typ, code: code, val: val}
		}
	}
}

// evRawEvent is a parsed evdev event (type, code, value).
type evRawEvent struct {
	typ  uint16
	code uint16
	val  int32
}

// ioctl helpers. These compute the ioctl numbers directly since
// golang.org/x/sys/unix doesn't expose the IOC macros.

func _IOC(dir, typ, nr, size uintptr) uint32 {
	return uint32((dir << 30) | (size << 16) | (typ << 8) | nr)
}

func evIOCGBIT(ev int, length int) uint32 {
	return _IOC(2, 'E', uintptr(0x20+ev), uintptr(length))
}

func evIOCGABS(abs int) uint32 {
	return _IOC(3, 'E', uintptr(0x40+abs), unsafe.Sizeof(inputAbsInfo{}))
}

func evIOCGNAME(length int) uint32 {
	return _IOC(2, 'E', 0x06, uintptr(length))
}
