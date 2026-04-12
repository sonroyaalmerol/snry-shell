package hyprland

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/sonroyaalmerol/snry-shell/internal/services/runner"
)

type Querier struct {
	cmd runner.Commander
}

func NewQuerier(cmd runner.Commander) *Querier {
	return &Querier{cmd: cmd}
}

func NewQuerierWithDefaults() *Querier {
	return NewQuerier(runner.NewCommander())
}

func (q *Querier) Clients() ([]HyprClient, error) {
	out, err := q.cmd.Run("clients", "-j")
	if err != nil {
		return nil, fmt.Errorf("hyprctl clients: %w", err)
	}
	var clients []HyprClient
	if err := json.Unmarshal(out, &clients); err != nil {
		return nil, fmt.Errorf("parse clients: %w", err)
	}
	return clients, nil
}

func (q *Querier) Workspaces() ([]HyprWorkspace, error) {
	out, err := q.cmd.Run("workspaces", "-j")
	if err != nil {
		return nil, fmt.Errorf("hyprctl workspaces: %w", err)
	}
	var ws []HyprWorkspace
	if err := json.Unmarshal(out, &ws); err != nil {
		return nil, fmt.Errorf("parse workspaces: %w", err)
	}
	return ws, nil
}

func (q *Querier) Monitors() ([]HyprMonitor, error) {
	out, err := q.cmd.Run("monitors", "-j")
	if err != nil {
		return nil, fmt.Errorf("hyprctl monitors: %w", err)
	}
	var monitors []HyprMonitor
	if err := json.Unmarshal(out, &monitors); err != nil {
		return nil, fmt.Errorf("parse monitors: %w", err)
	}
	return monitors, nil
}

func (q *Querier) Devices() (*HyprDevices, error) {
	out, err := q.cmd.Run("devices", "-j")
	if err != nil {
		return nil, fmt.Errorf("hyprctl devices: %w", err)
	}
	var devs HyprDevices
	if err := json.Unmarshal(out, &devs); err != nil {
		return nil, fmt.Errorf("parse devices: %w", err)
	}
	return &devs, nil
}

// ActiveKeymap returns the active keyboard layout from the main keyboard.
func (q *Querier) ActiveKeymap() (string, error) {
	devs, err := q.Devices()
	if err != nil {
		return "", err
	}
	for _, kb := range devs.Keyboard {
		if kb.Main {
			// e.g. "English (US)" → extract layout name
			parts := strings.Split(kb.ActiveKeymap, " (")
			if len(parts) > 0 {
				return strings.ToLower(parts[0]), nil
			}
			return strings.ToLower(kb.ActiveKeymap), nil
		}
	}
	return "", nil
}

// FocusWindow focuses a window by its Hyprland address.
func (q *Querier) FocusWindow(address string) error {
	_, err := q.cmd.Run("dispatch", "focuswindow", "address:"+address)
	return err
}

// SwitchXkbLayout cycles to the next keyboard layout.
func (q *Querier) SwitchXkbLayout() error {
	_, err := q.cmd.Run("dispatch", "switchxkblayout", "next")
	return err
}

// SwitchWorkspace switches to the given workspace ID.
func (q *Querier) SwitchWorkspace(id int) error {
	_, err := q.cmd.Run("dispatch", "workspace", strconv.Itoa(id))
	return err
}

// CloseActiveWindow kills the focused window.
func (q *Querier) CloseActiveWindow() error {
	_, err := q.cmd.Run("dispatch", "killactive")
	return err
}

// ToggleFullscreen toggles the focused window's fullscreen state.
func (q *Querier) ToggleFullscreen() error {
	_, err := q.cmd.Run("dispatch", "fullscreen", "1")
	return err
}

// ToggleFloating toggles the focused window between floating and tiled.
func (q *Querier) ToggleFloating() error {
	_, err := q.cmd.Run("dispatch", "togglefloating")
	return err
}

// MoveToWorkspace moves the focused window to the given workspace ID.
func (q *Querier) MoveToWorkspace(id int) error {
	_, err := q.cmd.Run("dispatch", "movetoworkspace", strconv.Itoa(id))
	return err
}

// ToggleSplit changes the split direction of the focused window.
func (q *Querier) ToggleSplit() error {
	_, err := q.cmd.Run("dispatch", "togglesplit")
	return err
}

// CloseWindow closes the window with the given address.
func (q *Querier) CloseWindow(address string) error {
	_, err := q.cmd.Run("dispatch", "closewindow", "address:"+address)
	return err
}

// ToggleFullscreenWindow toggles fullscreen for the window with the given address.
// Since fullscreen doesn't accept a window selector, we focus the window first.
func (q *Querier) ToggleFullscreenWindow(address string) error {
	// Focus the window first, then toggle fullscreen
	if _, err := q.cmd.Run("dispatch", "focuswindow", "address:"+address); err != nil {
		return err
	}
	_, err := q.cmd.Run("dispatch", "fullscreen", "0")
	return err
}

// ResizeWindow resizes the window with the given address to the exact pixel dimensions.
func (q *Querier) ResizeWindow(address string, w, h int) error {
	arg := fmt.Sprintf("exact %d %d, address:%s", w, h, address)
	_, err := q.cmd.Run("dispatch", "resizewindowpixel", arg)
	return err
}

// ToggleFloatingWindow toggles floating for the window with the given address.
func (q *Querier) ToggleFloatingWindow(address string) error {
	_, err := q.cmd.Run("dispatch", "togglefloating", "address:"+address)
	return err
}

// ToggleSplitWindow toggles split for the window with the given address.
// Since togglesplit doesn't accept a window selector, we focus the window first.
func (q *Querier) ToggleSplitWindow(address string) error {
	// Focus the window first, then toggle split
	if _, err := q.cmd.Run("dispatch", "focuswindow", "address:"+address); err != nil {
		return err
	}
	_, err := q.cmd.Run("dispatch", "togglesplit")
	return err
}

// MoveWindowToWorkspace moves the window with the given address to the specified workspace.
// Hyprland expects: movetoworkspace [workspace], address:[window]
func (q *Querier) MoveWindowToWorkspace(address string, id int) error {
	selector := fmt.Sprintf("%d, address:%s", id, address)
	_, err := q.cmd.Run("dispatch", "movetoworkspace", selector)
	return err
}

// ActiveWindow returns the currently focused window's class and title.
func (q *Querier) ActiveWindow() (HyprActiveWindow, error) {
	out, err := q.cmd.Run("activewindow", "-j")
	if err != nil {
		return HyprActiveWindow{}, fmt.Errorf("hyprctl activewindow: %w", err)
	}
	var win HyprActiveWindow
	if err := json.Unmarshal(out, &win); err != nil {
		return HyprActiveWindow{}, fmt.Errorf("parse activewindow: %w", err)
	}
	return win, nil
}

// SetKeyword sets a Hyprland config option at runtime.
func (q *Querier) SetKeyword(option, value string) error {
	log.Printf("[hyprland] keyword %s %s", option, value)
	out, err := q.cmd.Run("keyword", option, value)
	if err != nil {
		log.Printf("[hyprland] keyword error: %s, output: %s", err, string(out))
	}
	return err
}

// GetOption returns the current value of a Hyprland config option as a string.
// Output format: "int: 10\nset: true" or "str: value\nset: true"
func (q *Querier) GetOption(option string) (string, error) {
	log.Printf("[hyprland] getoption %s", option)
	out, err := q.cmd.Run("getoption", option)
	if err != nil {
		log.Printf("[hyprland] getoption error: %s, output: %s", err, string(out))
		return "", fmt.Errorf("hyprctl getoption %s: %w", option, err)
	}
	log.Printf("[hyprland] getoption %s raw output: %s", option, string(out))

	line := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	// Format: "type: value"
	if _, after, ok := strings.Cut(line, ": "); ok {
		return strings.TrimSpace(after), nil
	}
	return strings.TrimSpace(line), nil
}
