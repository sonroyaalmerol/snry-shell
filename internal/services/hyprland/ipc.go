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
	reader        EventReader
	bus           *bus.Bus
	windows       map[string]int    // window address → workspace ID
	windowClasses map[string]string // window address → class name
	workspaces    map[int]int       // wsID → window count
	wsClasses     map[int][]string  // wsID → window classes
}

func New(reader EventReader, b *bus.Bus) *Service {
	return &Service{
		reader:        reader,
		bus:           b,
		windows:       make(map[string]int),
		windowClasses: make(map[string]string),
		workspaces:    make(map[int]int),
		wsClasses:     make(map[int][]string),
	}
}

// SeedClients populates internal tracking maps from an initial client listing.
func (s *Service) SeedClients(clients []HyprClient) {
	for _, c := range clients {
		wsID := c.Workspace.ID
		addr := strings.TrimPrefix(c.Address, "0x")
		s.windows[addr] = wsID
		s.windowClasses[addr] = c.Class
		s.workspaces[wsID]++
		s.wsClasses[wsID] = append(s.wsClasses[wsID], c.Class)
	}
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
	case "workspacev2":
		ws := parseWorkspaceV2Event(data)
		ws.Occupied = s.workspaces[ws.ID] > 0
		ws.Classes = s.wsClasses[ws.ID]
		s.bus.Publish(bus.TopicWorkspaces, ws)

	case "workspace":
		ws := parseWorkspaceV1Event(data)
		ws.Occupied = s.workspaces[ws.ID] > 0
		ws.Classes = s.wsClasses[ws.ID]
		s.bus.Publish(bus.TopicWorkspaces, ws)

	case "activewindowv2":
		s.bus.Publish(bus.TopicActiveWindow, parseActiveWindowV2Event(data))

	case "activewindow":
		s.bus.Publish(bus.TopicActiveWindow, parseActiveWindowV1Event(data))

	case "openwindowv2":
		parts := strings.SplitN(data, ",", 5)
		if len(parts) >= 4 {
			addr := parts[0]
			var wsID int
			fmt.Sscanf(parts[1], "%d", &wsID)
			class := parts[3]
			s.windows[addr] = wsID
			s.windowClasses[addr] = class
			s.workspaces[wsID]++
			s.wsClasses[wsID] = append(s.wsClasses[wsID], class)
			s.publishWorkspace(wsID)
		}

	case "openwindow":
		parts := strings.SplitN(data, ",", 4)
		if len(parts) >= 3 {
			addr := parts[0]
			var wsID int
			fmt.Sscanf(parts[1], "%d", &wsID)
			class := parts[2]
			s.windows[addr] = wsID
			s.windowClasses[addr] = class
			s.workspaces[wsID]++
			s.wsClasses[wsID] = append(s.wsClasses[wsID], class)
			s.publishWorkspace(wsID)
		}

	case "closewindow", "closewindowv2":
		addr := strings.TrimSpace(data)
		if wsID, ok := s.windows[addr]; ok {
			class := s.windowClasses[addr]
			delete(s.windows, addr)
			delete(s.windowClasses, addr)
			s.workspaces[wsID]--
			s.removeWorkspaceClass(wsID, class)
			s.publishWorkspace(wsID)
		}

	case "movewindowv2":
		parts := strings.SplitN(data, ",", 3)
		if len(parts) >= 2 {
			addr := parts[0]
			var destID int
			fmt.Sscanf(parts[1], "%d", &destID)
			if srcID, ok := s.windows[addr]; ok {
				class := s.windowClasses[addr]
				s.workspaces[srcID]--
				s.removeWorkspaceClass(srcID, class)
				s.publishWorkspace(srcID)
			}
			s.windows[addr] = destID
			s.workspaces[destID]++
			s.wsClasses[destID] = append(s.wsClasses[destID], s.windowClasses[addr])
			s.publishWorkspace(destID)
		}

	case "movewindow":
		// v1: "WINDOWADDRESS,WORKSPACENAME" — resolve name to ID
		parts := strings.SplitN(data, ",", 2)
		if len(parts) == 2 {
			addr := parts[0]
			wsName := parts[1]
			if srcID, ok := s.windows[addr]; ok {
				class := s.windowClasses[addr]
				s.workspaces[srcID]--
				s.removeWorkspaceClass(srcID, class)
				s.publishWorkspace(srcID)
			}
			// Find workspace ID by name from existing entries
			var destID int
			for id, count := range s.workspaces {
				if count > 0 {
					// Try to match by parsing the name as ID
					if fmt.Sprintf("%d", id) == wsName {
						destID = id
						break
					}
				}
			}
			if destID == 0 {
				// Try parsing as number
				fmt.Sscanf(wsName, "%d", &destID)
			}
			s.windows[addr] = destID
			s.workspaces[destID]++
			s.wsClasses[destID] = append(s.wsClasses[destID], s.windowClasses[addr])
			s.publishWorkspace(destID)
		}

	case "layoutchanged":
		s.bus.Publish(bus.TopicKeyboard, data)

	case "fullscreen":
		s.bus.Publish(bus.TopicFullscreen, data == "1")
	}
}

// removeWorkspaceClass removes one occurrence of class from the workspace's class list.
func (s *Service) removeWorkspaceClass(wsID int, class string) {
	classes := s.wsClasses[wsID]
	for i, c := range classes {
		if c == class {
			s.wsClasses[wsID] = append(classes[:i], classes[i+1:]...)
			break
		}
	}
	if len(s.wsClasses[wsID]) == 0 {
		delete(s.wsClasses, wsID)
	}
}

func (s *Service) publishWorkspace(wsID int) {
	s.bus.Publish(bus.TopicWorkspaces, state.Workspace{
		ID:       wsID,
		Occupied: s.workspaces[wsID] > 0,
		Classes:  s.wsClasses[wsID],
	})
}

// parseWorkspaceV2Event parses "ID,NAME" format.
func parseWorkspaceV2Event(data string) state.Workspace {
	id, name, _ := strings.Cut(data, ",")
	ws := state.Workspace{Name: name, Active: true}
	fmt.Sscanf(id, "%d", &ws.ID)
	return ws
}

// parseWorkspaceV1Event parses just the workspace name (no comma).
func parseWorkspaceV1Event(data string) state.Workspace {
	ws := state.Workspace{Name: data, Active: true}
	fmt.Sscanf(data, "%d", &ws.ID)
	return ws
}

// parseActiveWindowV2Event parses "address,class,title" format.
func parseActiveWindowV2Event(data string) state.ActiveWindow {
	parts := strings.SplitN(data, ",", 3)
	if len(parts) < 3 {
		return state.ActiveWindow{}
	}
	return state.ActiveWindow{Class: parts[1], Title: parts[2]}
}

// parseActiveWindowV1Event parses "class,title" format.
func parseActiveWindowV1Event(data string) state.ActiveWindow {
	parts := strings.SplitN(data, ",", 2)
	if len(parts) < 2 {
		return state.ActiveWindow{}
	}
	return state.ActiveWindow{Class: parts[0], Title: parts[1]}
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
