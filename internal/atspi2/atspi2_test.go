package atspi2

import "testing"

func TestTextRoles(t *testing.T) {
	// Verify all expected text input roles are mapped
	expected := map[uint32]bool{
		roleEntry:       true,
		rolePasswordText: true,
		roleTerminal:    true,
		roleText:        true,
		roleSpinButton:  true,
		roleParagraph:   true,
		roleEditbar:     true,
		roleDateEditor:  true,
	}

	for role, want := range expected {
		if got := textRoles[role]; got != want {
			t.Errorf("textRoles[%d] = %v, want %v", role, got, want)
		}
	}
}

func TestTextRolesNonTextRolesExcluded(t *testing.T) {
	// Roles that should NOT trigger OSK
	nonTextRoles := []uint32{
		0,  // invalid
		1,  // alert
		10, // push button
		21, // document web
		32, // combo box (not inherently text-editable)
		42, // panel
	}

	for _, role := range nonTextRoles {
		if textRoles[role] {
			t.Errorf("textRoles[%d] should be false (non-text role)", role)
		}
	}
}

func TestRoleConstants(t *testing.T) {
	// Spot-check role constants against known AT-SPI2 values
	tests := []struct {
		name     string
		got      uint32
		expected uint32
	}{
		{"Entry", roleEntry, 70},
		{"PasswordText", rolePasswordText, 38},
		{"Terminal", roleTerminal, 58},
		{"Text", roleText, 59},
		{"SpinButton", roleSpinButton, 50},
		{"Paragraph", roleParagraph, 41},
		{"Editbar", roleEditbar, 68},
		{"DateEditor", roleDateEditor, 27},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("role%s = %d, want %d", tt.name, tt.got, tt.expected)
			}
		})
	}
}
