package touchgestures

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"unsafe"

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
	absX                = 0x00
	absY                = 0x01
	absMTSlot           = 0x2f
	absMTTrackingID     = 0x39
	absMTPositionX      = 0x35
	absMTPositionY      = 0x36
	absMTPressure       = 0x3a
	absMTTouchMajor     = 0x30
	absMTWidthMajor     = 0x32
)

// evdev key codes.
const (
	btnTouch       = 0x14a
	btnToolFinger  = 0x145
	btnToolDblTap  = 0x14d
	btnToolTriplTap = 0x14e
	btnToolQuadTap = 0x14f
	btnToolQuintTap = 0x14f + 1
)

// input device properties.
const (
	inputPropDirect  = 0x01
	inputPropPointer = 0x02
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
	Value    int32
	Minimum  int32
	Maximum  int32
	Fuzz     int32
	Flat     int32
	Resolution int32
}

// findTouchDevices parses /proc/bus/input/devices and returns paths for
// devices whose name contains "touch" and that report multitouch support.
func findTouchDevices() ([]string, error) {
	data, err := os.ReadFile("/proc/bus/input/devices")
	if err != nil {
		return nil, fmt.Errorf("read /proc/bus/input/devices: %w", err)
	}

	var devices []string
	var name string

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			name = ""
			continue
		}
		if strings.HasPrefix(line, "N: Name=") {
			name = strings.TrimPrefix(line, "N: Name=\"")
			name = strings.TrimSuffix(name, "\"")
		}
		if strings.HasPrefix(line, "H: Handlers=") && name != "" {
			lower := strings.ToLower(name)
			if strings.Contains(lower, "touch") && !strings.Contains(lower, "trackpad") && !strings.Contains(lower, "touchpad") {
				for _, field := range strings.Split(line, " ") {
					if strings.HasPrefix(field, "event") {
						devices = append(devices, "/dev/input/"+field)
					}
				}
			}
			name = ""
		}
	}
	return devices, scanner.Err()
}

// openTouchDevice opens an evdev device, verifies multitouch support via
// ioctl, and returns a TouchDevice with capability info.
func openTouchDevice(path string) (*TouchDevice, error) {
	fd, err := unix.Open(path, unix.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	// Check device has EV_ABS capability.
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
	// Check ABS_MT_POSITION_X bit is set.
	mtBit := absMTPositionX
	if evBits[mtBit/8]&(1<<(mtBit%8)) == 0 {
		unix.Close(fd)
		return nil, fmt.Errorf("%s: no multitouch support (ABS_MT_POSITION_X not set)", path)
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
		maxSlots = 10 // fallback
	}

	// Get X range.
	var xInfo inputAbsInfo
	unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(evIOCGABS(absMTPositionX)), uintptr(unsafe.Pointer(&xInfo)))

	// Get Y range.
	var yInfo inputAbsInfo
	unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(evIOCGABS(absMTPositionY)), uintptr(unsafe.Pointer(&yInfo)))

	f := os.NewFile(uintptr(fd), path)

	// Read device name from the device.
	var devName [256]byte
	_, _, _ = unix.Syscall(
		unix.SYS_IOCTL, uintptr(fd),
		uintptr(evIOCGNAME(256)),
		uintptr(unsafe.Pointer(&devName[0])),
	)
	name := strings.TrimRight(string(devName[:]), "\x00")

	return &TouchDevice{
		Path:     path,
		Name:     name,
		File:     f,
		MaxSlots: maxSlots,
		XRange:   [2]int32{xInfo.Minimum, xInfo.Maximum},
		YRange:   [2]int32{yInfo.Minimum, yInfo.Maximum},
	}, nil
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

// _IOC encodes an ioctl number: (dir << 30) | (size << 16) | (type << 8) | nr
// dir: READ=2, WRITE=1, READ|WRITE=3.
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
