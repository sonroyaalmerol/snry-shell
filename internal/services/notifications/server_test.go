package notifications_test

import (
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/notifications"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

type fakePublisher struct {
	events []bus.Event
}

func (f *fakePublisher) Publish(topic bus.Topic, data any) {
	f.events = append(f.events, bus.Event{Topic: topic, Data: data})
}

func TestNotifyPublishesEvent(t *testing.T) {
	fp := &fakePublisher{}
	srv := notifications.New(fp)

	id, dbusErr := srv.Notify("firefox", 0, "", "Title", "Body", nil, nil, -1)
	if dbusErr != nil {
		t.Fatal(dbusErr)
	}
	if id != 1 {
		t.Fatalf("expected id 1, got %d", id)
	}
	if len(fp.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(fp.events))
	}
	n := fp.events[0].Data.(state.Notification)
	if n.Summary != "Title" {
		t.Fatalf("unexpected summary: %q", n.Summary)
	}
	if n.AppName != "firefox" {
		t.Fatalf("unexpected appName: %q", n.AppName)
	}
	if n.Body != "Body" {
		t.Fatalf("unexpected body: %q", n.Body)
	}
}

func TestNotifyIncrementsID(t *testing.T) {
	fp := &fakePublisher{}
	srv := notifications.New(fp)

	id1, _ := srv.Notify("app", 0, "", "First", "", nil, nil, -1)
	id2, _ := srv.Notify("app", 0, "", "Second", "", nil, nil, -1)
	if id1 == id2 {
		t.Fatal("IDs should be unique")
	}
	if id2 != id1+1 {
		t.Fatalf("expected sequential IDs, got %d and %d", id1, id2)
	}
}

func TestNotifyReplacesID(t *testing.T) {
	fp := &fakePublisher{}
	srv := notifications.New(fp)

	id, _ := srv.Notify("app", 42, "", "Replaced", "", nil, nil, -1)
	if id != 42 {
		t.Fatalf("expected replacesID 42 to be returned, got %d", id)
	}
}

func TestNotifyUrgencyHint(t *testing.T) {
	fp := &fakePublisher{}
	srv := notifications.New(fp)

	hints := map[string]dbus.Variant{
		"urgency": dbus.MakeVariant(byte(2)),
	}
	srv.Notify("app", 0, "", "Critical", "", nil, hints, -1) //nolint:errcheck

	n := fp.events[0].Data.(state.Notification)
	if n.Urgency != 2 {
		t.Fatalf("expected urgency 2, got %d", n.Urgency)
	}
}

func TestGetCapabilities(t *testing.T) {
	srv := notifications.New(&fakePublisher{})
	caps, err := srv.GetCapabilities()
	if err != nil {
		t.Fatal(err)
	}
	if len(caps) == 0 {
		t.Fatal("expected non-empty capabilities")
	}
}

func TestGetServerInformation(t *testing.T) {
	srv := notifications.New(&fakePublisher{})
	name, vendor, version, spec, err := srv.GetServerInformation()
	if err != nil {
		t.Fatal(err)
	}
	if name == "" || vendor == "" || version == "" || spec == "" {
		t.Fatal("expected non-empty server information")
	}
}
