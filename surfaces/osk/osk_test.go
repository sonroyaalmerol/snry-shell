package osk

import "testing"

func TestActiveChar(t *testing.T) {
	tests := []struct {
		name    string
		normal  string
		shifted string
		shift   bool
		caps    bool
		want    string
	}{
		// Lowercase letter, no modifiers
		{"lower_no_mod", "a", "A", false, false, "a"},
		// Lowercase letter, shift only
		{"lower_shift", "a", "A", true, false, "A"},
		// Lowercase letter, caps only
		{"lower_caps", "a", "A", false, true, "A"},
		// Lowercase letter, both shift and caps (cancel out)
		{"lower_shift_caps", "a", "A", true, true, "a"},
		// Symbol with shift variant
		{"symbol_shift", "1", "!", true, false, "!"},
		// Symbol without shift
		{"symbol_no_shift", "1", "!", false, false, "1"},
		// Letter with no shifted variant (falls through to ToUpper)
		{"letter_no_shifted", "z", "", true, false, "Z"},
		// Caps with no shifted variant
		{"letter_caps_no_shifted", "z", "", false, true, "Z"},
		// Both mods cancel for letter without shifted
		{"letter_both_no_shifted", "z", "", true, true, "z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &OSK{shift: tt.shift, caps: tt.caps}
			kb := &keyButton{normal: tt.normal, shifted: tt.shifted}
			got := o.activeChar(kb)
			if got != tt.want {
				t.Errorf("activeChar(normal=%q, shifted=%q, shift=%v, caps=%v) = %q, want %q",
					tt.normal, tt.shifted, tt.shift, tt.caps, got, tt.want)
			}
		})
	}
}

func TestScheduleFocusUpdateSetsTimer(t *testing.T) {
	tests := []struct {
		name     string
		want     bool
		hasTouch bool
	}{
		{"text_with_touch", true, true},
		{"text_no_touch", true, false},
		{"nontext_with_touch", false, true},
		{"nontext_no_touch", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &OSK{hasTouch: tt.hasTouch}
			o.scheduleFocusUpdate(tt.want)
			if o.debounce == nil {
				t.Errorf("scheduleFocusUpdate(%v) should set debounce timer", tt.want)
			}
			o.debounce.Stop()
		})
	}
}
