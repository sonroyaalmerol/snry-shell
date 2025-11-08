package hyprland

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Commander abstracts running hyprctl so tests can inject a fake.
type Commander interface {
	Run(args ...string) ([]byte, error)
}

type hyprctlCommander struct{}

func (h hyprctlCommander) Run(args ...string) ([]byte, error) {
	return exec.Command("hyprctl", args...).Output()
}

// NewCommander returns a Commander backed by hyprctl.
func NewCommander() Commander {
	return hyprctlCommander{}
}

// Querier issues hyprctl JSON queries on demand.
type Querier struct {
	cmd Commander
}

func NewQuerier(cmd Commander) *Querier {
	return &Querier{cmd: cmd}
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
