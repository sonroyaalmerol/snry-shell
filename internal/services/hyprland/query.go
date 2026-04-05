package hyprland

import (
	"encoding/json"
	"fmt"
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

// SetKeyword sets a Hyprland config option at runtime.
func (q *Querier) SetKeyword(option, value string) error {
	_, err := q.cmd.Run("keyword", option, value)
	return err
}

// hyprOption is the JSON output of `hyprctl getoption <name>`.
type hyprOption struct {
	Str   string  `json:"str"`
	Int   int     `json:"int"`
	Float float64 `json:"float"`
}

// GetOption returns the current value of a Hyprland config option as a string.
func (q *Querier) GetOption(option string) (string, error) {
	out, err := q.cmd.Run("getoption", option)
	if err != nil {
		return "", fmt.Errorf("hyprctl getoption %s: %w", option, err)
	}
	var opt hyprOption
	if err := json.Unmarshal(out, &opt); err != nil {
		return "", fmt.Errorf("parse getoption %s: %w", option, err)
	}
	if opt.Str != "" {
		return opt.Str, nil
	}
	if opt.Int != 0 {
		return strconv.Itoa(opt.Int), nil
	}
	return fmt.Sprintf("%g", opt.Float), nil
}
