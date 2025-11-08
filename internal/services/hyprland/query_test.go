package hyprland_test

import (
	"testing"

	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
)

type fakeCommander struct {
	responses map[string][]byte
	err       error
}

func (f *fakeCommander) Run(args ...string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.responses[args[0]], nil
}

func TestQuerierClients(t *testing.T) {
	fake := &fakeCommander{
		responses: map[string][]byte{
			"clients": []byte(`[{"class":"foot","title":"Terminal","address":"0x1"}]`),
		},
	}
	q := hyprland.NewQuerier(fake)
	clients, err := q.Clients()
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	if clients[0].Class != "foot" {
		t.Fatalf("expected class 'foot', got %q", clients[0].Class)
	}
	if clients[0].Title != "Terminal" {
		t.Fatalf("expected title 'Terminal', got %q", clients[0].Title)
	}
}

func TestQuerierWorkspaces(t *testing.T) {
	fake := &fakeCommander{
		responses: map[string][]byte{
			"workspaces": []byte(`[{"id":1,"name":"1"},{"id":2,"name":"code"}]`),
		},
	}
	q := hyprland.NewQuerier(fake)
	ws, err := q.Workspaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(ws))
	}
	if ws[1].Name != "code" {
		t.Fatalf("expected name 'code', got %q", ws[1].Name)
	}
}

func TestQuerierMonitors(t *testing.T) {
	fake := &fakeCommander{
		responses: map[string][]byte{
			"monitors": []byte(`[{"id":0,"name":"DP-1","activeWorkspace":{"id":1,"name":"1"}}]`),
		},
	}
	q := hyprland.NewQuerier(fake)
	monitors, err := q.Monitors()
	if err != nil {
		t.Fatal(err)
	}
	if len(monitors) != 1 || monitors[0].Name != "DP-1" {
		t.Fatalf("unexpected monitors: %+v", monitors)
	}
	if monitors[0].ActiveWorkspace.ID != 1 {
		t.Fatalf("unexpected active workspace: %+v", monitors[0].ActiveWorkspace)
	}
}

func TestQuerierInvalidJSON(t *testing.T) {
	fake := &fakeCommander{
		responses: map[string][]byte{
			"clients": []byte(`not json`),
		},
	}
	q := hyprland.NewQuerier(fake)
	_, err := q.Clients()
	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}
