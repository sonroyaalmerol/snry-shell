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
	return &socketReader{scanner: bufio.NewScanner(r)}
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
	reader          EventReader
	bus             *bus.Bus
	// occupiedWorkspaces tracks which workspace IDs currently have windows.
	occupiedWorkspaces map[int]int // wsID → window count
}

func New(reader EventReader, b *bus.Bus) *Service {
	return &Service{
		reader:             reader,
		bus:                b,
		occupiedWorkspaces: make(map[int]int),
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
	case "workspace", "workspacev2":
		ws := parseWorkspaceEvent(data)
		ws.Occupied = s.occupiedWorkspaces[ws.ID] > 0
		s.bus.Publish(bus.TopicWorkspaces, ws)

	case "activewindow", "activewindowv2":
		s.bus.Publish(bus.TopicActiveWindow, parseActiveWindowEvent(data))

	case "openwindow":
		// Format: "windowaddress,workspaceid,class,title"
		parts := strings.SplitN(data, ",", 4)
		if len(parts) >= 2 {
			var wsID int
			fmt.Sscanf(parts[1], "%d", &wsID)
			s.occupiedWorkspaces[wsID]++
			s.bus.Publish(bus.TopicWorkspaces, state.Workspace{
				ID:       wsID,
				Occupied: true,
			})
		}

	case "closewindow":
		// Re-query occupancy after a close — we don't have the workspace here,
		// so a full workspace refresh will happen on the next workspace event.
		// Decrement whichever workspace had windows if tracked.
		for wsID, count := range s.occupiedWorkspaces {
			if count > 0 {
				s.occupiedWorkspaces[wsID]--
				if s.occupiedWorkspaces[wsID] == 0 {
					s.bus.Publish(bus.TopicWorkspaces, state.Workspace{
						ID:       wsID,
						Occupied: false,
					})
				}
			}
		}

	case "movewindow":
		// Format: "windowaddress,workspaceid"
		parts := strings.SplitN(data, ",", 2)
		if len(parts) == 2 {
			var wsID int
			fmt.Sscanf(parts[1], "%d", &wsID)
			s.occupiedWorkspaces[wsID]++
		}
	}
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
