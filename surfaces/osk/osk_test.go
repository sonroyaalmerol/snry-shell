package osk

import "testing"

func TestIsTextInputWindow(t *testing.T) {
	tests := []struct {
		class string
		want  bool
	}{
		// Terminals
		{"kitty", true},
		{"Alacritty", true},
		{"WezTerm", true},
		{"foot", true},
		{"Ghostty", true},
		// Browsers
		{"firefox", true},
		{"Chromium", true},
		{"brave-browser", true},
		// Editors
		{"code", true},
		{"code-oss", true},
		{"Cursor", true},
		{"Neovide", true},
		// Chat
		{"TelegramDesktop", true},
		{"discord", true},
		{"vesktop", true},
		{"Signal", true},
		// Other
		{"obsidian", true},
		{"spotify", true},
		{"thunderbird", true},
		// Non-text apps
		{"thunar", false},
		{"nautilus", false},
		{"dolphin", false},
		{"pavucontrol", false},
		{"mpv", false},
		{"imv", false},
		{"gimp", false},
		{"blender", false},
		// Edge cases
		{"", false},
		{"code-insiders", true}, // contains "code"
	}

	for _, tt := range tests {
		t.Run(tt.class, func(t *testing.T) {
			got := isTextInputWindow(tt.class)
			if got != tt.want {
				t.Errorf("isTextInputWindow(%q) = %v, want %v", tt.class, got, tt.want)
			}
		})
	}
}

func TestIsHexAddress(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"", true},
		{"0x", true},
		{"0x1a2b3c", true},
		{"deadbeef", true},
		{"DEADBEEF", true},
		{"0xDEADBEEF", true},
		{"0X1234", true},
		// Not hex
		{"kitty", false},
		{"0xg", false},
		{"firefox", false},
		{"hello world", false},
		{"0x1234z", false},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := isHexAddress(tt.s)
			if got != tt.want {
				t.Errorf("isHexAddress(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestIsShellSurface(t *testing.T) {
	tests := []struct {
		class string
		want  bool
	}{
		{"snry-bar", true},
		{"snry-osk", true},
		{"snry-lock", true},
		{"Snry-Bar", true},
		{"SNRY-OSK", true},
		// Not shell surfaces
		{"kitty", false},
		{"firefox", false},
		{"", false},
		{"snry", false},       // prefix without dash
		{"not-snry-bar", false}, // contains but doesn't start with snry-
	}

	for _, tt := range tests {
		t.Run(tt.class, func(t *testing.T) {
			got := isShellSurface(tt.class)
			if got != tt.want {
				t.Errorf("isShellSurface(%q) = %v, want %v", tt.class, got, tt.want)
			}
		})
	}
}

func TestActiveChar(t *testing.T) {
	tests := []struct {
		name   string
		normal string
		shifted string
		shift  bool
		caps   bool
		want   string
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

func TestScheduleFocusUpdateFilters(t *testing.T) {
	// Verify that hex addresses and shell surfaces are filtered out
	// (they should not cause a panic and should not set a debounce timer)
	tests := []struct {
		name  string
		class string
	}{
		{"hex_address", "0xdeadbeef"},
		{"empty_string", ""},
		{"shell_bar", "snry-bar"},
		{"shell_osk", "snry-osk"},
		{"shell_upper", "Snry-Lock"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &OSK{hasTouch: true}
			o.scheduleFocusUpdate(tt.class)
			if o.debounce != nil {
				t.Errorf("scheduleFocusUpdate(%q) should not set debounce for filtered classes", tt.class)
			}
		})
	}
}

func TestScheduleFocusUpdateSetsTimer(t *testing.T) {
	// Non-filtered classes (including AT-SPI2) should always set a debounce timer,
	// since the timer callback handles both show and hide transitions.
	tests := []struct {
		name     string
		class    string
		hasTouch bool
	}{
		{"atspi2_text_with_touch", "atspi2:text", true},
		{"atspi2_text_no_touch", "atspi2:text", false},
		{"atspi2_nontext_with_touch", "atspi2:non-text", true},
		{"atspi2_nontext_no_touch", "atspi2:non-text", false},
		{"window_text_kitty", "kitty", true},
		{"window_text_firefox", "firefox", true},
		{"window_nontext_thunar", "thunar", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &OSK{hasTouch: tt.hasTouch}
			o.scheduleFocusUpdate(tt.class)
			if o.debounce == nil {
				t.Errorf("scheduleFocusUpdate(%q) should set debounce timer", tt.class)
			}
			// Stop the timer to avoid goroutine leak
			o.debounce.Stop()
		})
	}
}
