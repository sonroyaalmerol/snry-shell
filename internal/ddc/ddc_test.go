package ddc

import (
	"os"
	"testing"
)

func TestBuildGetVCP(t *testing.T) {
	req := buildGetVCP(0x10)
	if len(req) != 5 {
		t.Fatalf("length: got %d, want 5", len(req))
	}
	// Checksum = XOR(0x6E, 0x51, 0x82, 0x01, 0x10)
	wantChecksum := ddcChecksum([]byte{0x6E, 0x51, 0x82, 0x01, 0x10})
	expected := []byte{0x51, 0x82, 0x01, 0x10, wantChecksum}
	for i, b := range expected {
		if req[i] != b {
			t.Errorf("byte %d: got 0x%02x, want 0x%02x", i, req[i], b)
		}
	}
}

func TestBuildSetVCP(t *testing.T) {
	req := buildSetVCP(0x10, 50)
	if len(req) != 7 {
		t.Fatalf("length: got %d, want 7", len(req))
	}
	wantChecksum := ddcChecksum([]byte{0x6E, 0x51, 0x84, 0x03, 0x10, 0x00, 0x32})
	expected := []byte{0x51, 0x84, 0x03, 0x10, 0x00, 0x32, wantChecksum}
	for i, b := range expected {
		if req[i] != b {
			t.Errorf("byte %d: got 0x%02x, want 0x%02x", i, req[i], b)
		}
	}
}

func TestDdcChecksum(t *testing.T) {
	tests := []struct {
		data []byte
		want byte
	}{
		{[]byte{0x6E, 0x51, 0x82, 0x01, 0x10}, ddcChecksum([]byte{0x6E, 0x51, 0x82, 0x01, 0x10})},
		{[]byte{0x00}, 0x00},
		{[]byte{0xFF, 0xFF}, 0x00},
		{[]byte{0x01}, 0x01},
	}
	for _, tt := range tests {
		got := ddcChecksum(tt.data)
		if got != tt.want {
			t.Errorf("checksum(%x) = 0x%02x, want 0x%02x", tt.data, got, tt.want)
		}
	}
}

func TestParseGetVCPResponse(t *testing.T) {
	// Simulated response for brightness=20, max=100
	buf := []byte{0x6E, 0x89, 0x02, 0x00, 0x10, 0x00, 0x00, 0x64, 0x00, 0x14, 0xE5}
	v, err := parseGetVCPResponse(buf)
	if err != nil {
		t.Fatal(err)
	}
	if v.Code != 0x10 {
		t.Errorf("code: got 0x%02x, want 0x10", v.Code)
	}
	if v.Current != 20 {
		t.Errorf("current: got %d, want 20", v.Current)
	}
	if v.Max != 100 {
		t.Errorf("max: got %d, want 100", v.Max)
	}
}

func TestParseGetVCPResponseShort(t *testing.T) {
	_, err := parseGetVCPResponse([]byte{0x6E, 0x89})
	if err == nil {
		t.Fatal("expected error for short response")
	}
}

func TestParseGetVCPResponseBadSrc(t *testing.T) {
	buf := []byte{0x00, 0x89, 0x02, 0x00, 0x10, 0x00, 0x00, 0x64, 0x00, 0x14, 0x00}
	_, err := parseGetVCPResponse(buf)
	if err == nil {
		t.Fatal("expected error for bad src")
	}
}

func TestBusFromDRM(t *testing.T) {
	buses, err := BusFromDRM()
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("no /sys/class/drm (not Linux or no DRM)")
		}
		t.Fatal(err)
	}
	if len(buses) == 0 {
		t.Fatal("expected at least one bus")
	}
	t.Logf("found %d I2C buses: %v", len(buses), buses)
}

func TestGetBrightnessReal(t *testing.T) {
	if os.Getenv("DDC_INTEGRATION") == "" {
		t.Skip("set DDC_INTEGRATION=1 to run hardware test")
	}
	v, err := GetVCP(0x10)
	if err != nil {
		t.Fatalf("GetVCP(0x10): %v", err)
	}
	t.Logf("brightness: current=%d, max=%d", v.Current, v.Max)
	if v.Max == 0 {
		t.Error("max brightness is 0")
	}
	// Verify current brightness matches ddcutil output (should be 20 on this machine).
	if v.Current != 20 {
		t.Errorf("current brightness: got %d, want 20 (check ddcutil getvcp 10)", v.Current)
	}
	if v.Max != 100 {
		t.Errorf("max brightness: got %d, want 100", v.Max)
	}
}
