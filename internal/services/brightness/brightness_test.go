package brightness_test

import (
	"regexp"
	"testing"
)

// ddcutil output: "VCP code 0x10 (Brightness): current value =    20, max value =   100"
var ddcutilRe = regexp.MustCompile(`current value\s*=\s*(\d+),\s*max value\s*=\s*(\d+)`)

func TestDdcutilRegex(t *testing.T) {
	tests := []struct {
		name   string
		out    string
		wantOk bool
	}{
		{"typical", "VCP code 0x10 (Brightness                    ): current value =    20, max value =   100\n", true},
		{"zero", "VCP code 0x10 (Brightness                    ): current value =    0, max value =   100\n", true},
		{"max", "VCP code 0x10 (Brightness                    ): current value =   100, max value =   100\n", true},
		{"tight", "VCP code 0x10 (Brightness): current value=50,max value=100\n", true},
		{"garbage", "no brightness here\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ddcutilRe.FindStringSubmatch(tt.out)
			if tt.wantOk && m == nil {
				t.Fatal("expected match")
			}
			if !tt.wantOk && m != nil {
				t.Fatalf("unexpected match: %v", m)
			}
		})
	}
}
