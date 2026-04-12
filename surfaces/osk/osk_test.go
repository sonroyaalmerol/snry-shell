package osk

import "testing"

func TestActiveCharLocked(t *testing.T) {
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
		// Letter with no shifted variant — returns normal
		{"letter_no_shifted", "z", "", true, false, "z"},
		// Caps with no shifted variant — returns normal
		{"letter_caps_no_shifted", "z", "", false, true, "z"},
		// Both mods cancel — returns normal
		{"letter_both_no_shifted", "z", "", true, true, "z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &OSK{}
			o.mu.Lock()
			o.shift = tt.shift
			o.caps = tt.caps
			kb := &keyButton{normal: tt.normal, shifted: tt.shifted}
			got := o.activeCharLocked(kb)
			o.mu.Unlock()
			if got != tt.want {
				t.Errorf("activeCharLocked(normal=%q, shifted=%q, shift=%v, caps=%v) = %q, want %q",
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
			o.mu.Lock()
			timer := o.debounce
			o.mu.Unlock()
			if timer == nil {
				t.Errorf("scheduleFocusUpdate(%v) should set debounce timer", tt.want)
			}
			timer.Stop()
		})
	}
}
