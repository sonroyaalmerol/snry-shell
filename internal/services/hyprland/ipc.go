package hyprland

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// EventReader abstracts the Hyprland IPC socket so tests can inject a fake.
type EventReader interface {
	Read() (event, data string, err error)
}

type socketReader struct {
	scanner *bufio.Scanner
}

// NewSocketReader wraps any io.Reader (real socket or test buffer) as an EventReader.
func NewSocketReader(r io.Reader) EventReader {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 1024), 1024*1024)
	return &socketReader{scanner: s}
}

func (s *socketReader) Read() (string, string, error) {
	if !s.scanner.Scan() {
		if err := s.scanner.Err(); err != nil {
			return "", "", err
		}
		return "", "", io.EOF
	}
	line := s.scanner.Text()
	event, data, ok := strings.Cut(line, ">>")
	if !ok {
		return "", "", nil
	}
	return event, data, nil
}

// Service listens to Hyprland socket2 events and publishes them on the bus.
type Service struct {
	reader         EventReader
	bus            *bus.Bus
	windows        map[string]int    // window address → workspace ID
	windowClasses  map[string]string // window address → class name
	workspaces     map[int]int       // wsID → window count
	workspaceIcons map[int]string    // wsID → first window class
}

func New(reader EventReader, b *bus.Bus) *Service {
	return &Service{
		reader:         reader,
		bus:            b,
		windows:        make(map[string]int),
		windowClasses:  make(map[string]string),
		workspaces:     make(map[int]int),
		workspaceIcons: make(map[int]string),
	}
}

// SeedClients populates internal tracking maps from an initial client listing.
// Call before Run to ensure workspace events carry correct state from startup.
func (s *Service) SeedClients(clients []HyprClient) {
	for _, c := range clients {
		wsID := c.Workspace.ID
		addr := strings.TrimPrefix(c.Address, "0x")
		s.windows[addr] = wsID
		s.windowClasses[addr] = c.Class
		s.workspaces[wsID]++
		if _, ok := s.workspaceIcons[wsID]; !ok {
			s.workspaceIcons[wsID] = c.Class
		}
	}
}

// firstClassOnWorkspace returns the class of the first window found on the
// given workspace, or "" if empty.
func (s *Service) firstClassOnWorkspace(wsID int) string {
	for addr, wID := range s.windows {
		if wID == wsID {
			if class, ok := s.windowClasses[addr]; ok {
				return class
			}
		}
	}
	return ""
}

func (s *Service) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		event, data, err := s.reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("hyprland ipc: %w", err)
		}
		s.handleEvent(event, data)
	}
}

func (s *Service) handleEvent(event, data string) {
	switch event {
	case "workspace", "workspacev2":
		ws := parseWorkspaceEvent(data)
		ws.Occupied = s.workspaces[ws.ID] > 0
		ws.Icon = s.workspaceIcons[ws.ID]
		s.bus.Publish(bus.TopicWorkspaces, ws)

	case "activewindow", "activewindowv2":
		s.bus.Publish(bus.TopicActiveWindow, parseActiveWindowEvent(data))

	case "openwindow", "openwindowv2":
		// Format: "windowaddress,workspaceid,class,title"
		parts := strings.SplitN(data, ",", 4)
		if len(parts) >= 3 {
			addr := parts[0]
			var wsID int
			fmt.Sscanf(parts[1], "%d", &wsID)
			class := parts[2]
			s.windows[addr] = wsID
			s.windowClasses[addr] = class
			s.workspaces[wsID]++
			if _, ok := s.workspaceIcons[wsID]; !ok {
				s.workspaceIcons[wsID] = class
			}
			s.publishWorkspace(wsID)
		}

	case "closewindow", "closewindowv2":
		addr := strings.TrimSpace(data)
		if wsID, ok := s.windows[addr]; ok {
			delete(s.windows, addr)
			delete(s.windowClasses, addr)
			s.workspaces[wsID]--
			if s.workspaces[wsID] <= 0 {
				delete(s.workspaces, wsID)
				delete(s.workspaceIcons, wsID)
			} else {
				s.workspaceIcons[wsID] = s.firstClassOnWorkspace(wsID)
			}
			s.publishWorkspace(wsID)
		}

	case "movewindow", "movewindowv2":
		// Format: "windowaddress,workspaceid"
		parts := strings.SplitN(data, ",", 2)
		if len(parts) == 2 {
			addr := parts[0]
			var destID int
			fmt.Sscanf(parts[1], "%d", &destID)
			if srcID, ok := s.windows[addr]; ok {
				s.workspaces[srcID]--
				if s.workspaces[srcID] <= 0 {
					delete(s.workspaces, srcID)
					delete(s.workspaceIcons, srcID)
				} else {
					s.workspaceIcons[srcID] = s.firstClassOnWorkspace(srcID)
				}
				s.publishWorkspace(srcID)
			}
			s.windows[addr] = destID
			s.workspaces[destID]++
			if _, ok := s.workspaceIcons[destID]; !ok {
				if class, ok := s.windowClasses[addr]; ok {
					s.workspaceIcons[destID] = class
				}
			}
			s.publishWorkspace(destID)
		}

	case "layoutchanged":
		s.bus.Publish(bus.TopicKeyboard, data)

	case "fullscreen":
		s.bus.Publish(bus.TopicFullscreen, data == "1")
	}
}

func (s *Service) publishWorkspace(wsID int) {
	s.bus.Publish(bus.TopicWorkspaces, state.Workspace{
		ID:       wsID,
		Occupied: s.workspaces[wsID] > 0,
		Icon:     s.workspaceIcons[wsID],
	})
}

func parseWorkspaceEvent(data string) state.Workspace {
	// workspacev2 format: "id,name"
	id, name, _ := strings.Cut(data, ",")
	ws := state.Workspace{Name: name, Active: true}
	fmt.Sscanf(id, "%d", &ws.ID)
	return ws
}

func parseActiveWindowEvent(data string) state.ActiveWindow {
	class, title, _ := strings.Cut(data, ",")
	return state.ActiveWindow{Class: class, Title: title}
}

// SocketPath returns the path to Hyprland's event socket.
func SocketPath() string {
	his := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	xdgRuntime := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntime == "" {
		xdgRuntime = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return fmt.Sprintf("%s/hypr/%s/.socket2.sock", xdgRuntime, his)
}
