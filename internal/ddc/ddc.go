// Package ddc provides DDC/CI monitor control over I2C.
// It communicates directly with displays via Linux I2C ioctls,
// replacing external tools like ddcutil.
package ddc

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	ddcAddr = 0x37 // I2C slave address for DDC/CI

	// VCP commands.
	vcpGet = 0x01
	vcpSet = 0x03

	// I2C ioctls.
	iocRdwr = 0x0707
	iocMRd   = 0x0001

	readSleep = 40 * time.Millisecond
	writeTries = 3
)

// Kernel struct layouts for I2C_RDWR ioctl (amd64).
type i2cMsg struct {
	Addr  uint16
	Flags uint16
	Len   uint16
	_     uint16
	Buf   *byte
}

type i2cRdwrIoctlData struct {
	Msgs  *i2cMsg
	Nmsgs uint32
	_     uint32
}

// VCPValue holds the result of a VCP feature read.
type VCPValue struct {
	Code    byte
	Current uint16
	Max     uint16
}

// BusFromDRM discovers I2C bus numbers. It scans all /dev/i2c-* devices.
func BusFromDRM() ([]int, error) {
	devs, err := filepath.Glob("/dev/i2c-*")
	if err != nil {
		return nil, fmt.Errorf("scan /dev/i2c-*: %w", err)
	}
	var buses []int
	for _, dev := range devs {
		n, err := strconv.Atoi(filepath.Base(dev)[4:])
		if err != nil {
			continue
		}
		buses = append(buses, n)
	}
	if len(buses) == 0 {
		return nil, fmt.Errorf("no I2C buses found")
	}
	return buses, nil
}

// cachedBus stores the bus number that last responded to a VCP read.
// This avoids scanning all buses on every poll.
// cachedBusValid tracks whether cachedBus has been set.
var (
	cachedBus      int
	cachedBusValid bool
)

// GetVCP reads a VCP feature. Uses the cached bus if available,
// otherwise scans all buses.
func GetVCP(code byte) (VCPValue, error) {
	if cachedBusValid {
		v, err := getVCPBus(cachedBus, code)
		if err == nil {
			return v, nil
		}
	}
	return scanGetVCP(code)
}

func scanGetVCP(code byte) (VCPValue, error) {
	buses, err := BusFromDRM()
	if err != nil {
		return VCPValue{}, err
	}
	var lastErr error
	for _, bus := range buses {
		v, err := getVCPBus(bus, code)
		if err == nil {
			cachedBus = bus
				cachedBusValid = true
				return v, nil
		}
		lastErr = err
	}
	return VCPValue{}, fmt.Errorf("no monitor responded: %w", lastErr)
}

// SetVCP writes a VCP feature value. Tries the cached bus first,
// then scans all buses.
func SetVCP(code byte, value uint16) error {
	if cachedBusValid {
		if err := setVCPBus(cachedBus, code, value); err == nil {
			return nil
		}
	}
	buses, err := BusFromDRM()
	if err != nil {
		return err
	}
	var lastErr error
	for _, bus := range buses {
		if err := setVCPBus(bus, code, value); err == nil {
			cachedBus = bus
				cachedBusValid = true
				return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("no monitor responded: %w", lastErr)
}

func openBus(bus int) (int, error) {
	path := fmt.Sprintf("/dev/i2c-%d", bus)
	fd, err := unix.Open(path, unix.O_RDWR, 0)
	if err != nil {
		return -1, fmt.Errorf("open %s: %w", path, err)
	}
	return fd, nil
}

func i2cWrite(fd int, buf []byte) error {
	msg := i2cMsg{
		Addr:  ddcAddr,
		Flags: 0,
		Len:   uint16(len(buf)),
		Buf:   &buf[0],
	}
	data := i2cRdwrIoctlData{
		Msgs:  &msg,
		Nmsgs: 1,
	}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		uintptr(iocRdwr),
		uintptr(unsafe.Pointer(&data)),
	)
	if errno != 0 {
		return fmt.Errorf("i2c write: %w", errno)
	}
	return nil
}

func i2cRead(fd int, buf []byte) error {
	msg := i2cMsg{
		Addr:  ddcAddr,
		Flags: iocMRd,
		Len:   uint16(len(buf)),
		Buf:   &buf[0],
	}
	data := i2cRdwrIoctlData{
		Msgs:  &msg,
		Nmsgs: 1,
	}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		uintptr(iocRdwr),
		uintptr(unsafe.Pointer(&data)),
	)
	if errno != 0 {
		return fmt.Errorf("i2c read: %w", errno)
	}
	return nil
}

func ddcChecksum(data []byte) byte {
	var c byte
	for _, b := range data {
		c ^= b
	}
	return c
}

func buildGetVCP(code byte) []byte {
	data := []byte{0x6E, 0x51, 0x82, vcpGet, code}
	return append(data[1:], ddcChecksum(data))
}

func buildSetVCP(code byte, value uint16) []byte {
	data := []byte{0x6E, 0x51, 0x84, vcpSet, code, byte(value >> 8), byte(value)}
	return append(data[1:], ddcChecksum(data))
}

func parseGetVCPResponse(buf []byte) (VCPValue, error) {
	// Minimum response: src(1) len(1) type(1) result(1) code(1) vcp_type(1) max(2) cur(2) checksum(1) = 11 bytes
	if len(buf) < 11 {
		return VCPValue{}, fmt.Errorf("response too short: %d bytes", len(buf))
	}
	// Validate: src=0x6E, type=0x02 (Get VCP Reply), result=0x00 (success)
	if buf[0] != 0x6E {
		return VCPValue{}, fmt.Errorf("unexpected src: 0x%02x", buf[0])
	}
	if buf[2] != 0x02 {
		return VCPValue{}, fmt.Errorf("unexpected response type: 0x%02x", buf[2])
	}
	if buf[3] != 0x00 {
		return VCPValue{}, fmt.Errorf("VCP error: 0x%02x", buf[3])
	}
	max := uint16(buf[6])<<8 | uint16(buf[7])
	cur := uint16(buf[8])<<8 | uint16(buf[9])
	return VCPValue{Code: buf[4], Current: cur, Max: max}, nil
}

func getVCPBus(bus int, code byte) (VCPValue, error) {
	fd, err := openBus(bus)
	if err != nil {
		return VCPValue{}, err
	}
	defer unix.Close(fd)

	req := buildGetVCP(code)

	var lastErr error
	for i := range writeTries {
		if err := i2cWrite(fd, req); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 10 * time.Millisecond)
			continue
		}
		time.Sleep(readSleep)
		resp := make([]byte, 32)
		if err := i2cRead(fd, resp); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 10 * time.Millisecond)
			continue
		}
		return parseGetVCPResponse(resp)
	}
	return VCPValue{}, lastErr
}

func setVCPBus(bus int, code byte, value uint16) error {
	fd, err := openBus(bus)
	if err != nil {
		return err
	}
	defer unix.Close(fd)

	req := buildSetVCP(code, value)
	return i2cWrite(fd, req)
}
