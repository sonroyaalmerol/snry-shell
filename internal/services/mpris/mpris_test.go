package mpris_test

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/mpris"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// Verify noopBusObject satisfies dbus.BusObject at compile time.
var _ dbus.BusObject = noopBusObject{}

// fakeDBusConn implements mpris.DBusConn for tests.
type fakeDBusConn struct {
	signalCh chan *dbus.Signal
}

func newFakeConn() *fakeDBusConn {
	return &fakeDBusConn{signalCh: make(chan *dbus.Signal, 8)}
}

func (f *fakeDBusConn) Signal(ch chan<- *dbus.Signal) {
	go func() {
		for sig := range f.signalCh {
			ch <- sig
		}
	}()
}

func (f *fakeDBusConn) BusObject() dbus.BusObject                     { return nil }
func (f *fakeDBusConn) Object(string, dbus.ObjectPath) dbus.BusObject { return nil }
func (f *fakeDBusConn) AddMatchSignal(opts ...dbus.MatchOption) error { return nil }

// fakeBusObject wraps BusObject calls so Signal() doesn't nil-panic in tests.
type noopBusObject struct{}

func (n noopBusObject) Call(method string, flags dbus.Flags, args ...any) *dbus.Call {
	return &dbus.Call{}
}
func (n noopBusObject) CallWithContext(ctx context.Context, method string, flags dbus.Flags, args ...any) *dbus.Call {
	return &dbus.Call{}
}
func (n noopBusObject) Go(method string, flags dbus.Flags, ch chan *dbus.Call, args ...any) *dbus.Call {
	return &dbus.Call{}
}
func (n noopBusObject) GoWithContext(ctx context.Context, method string, flags dbus.Flags, ch chan *dbus.Call, args ...any) *dbus.Call {
	return &dbus.Call{}
}
func (n noopBusObject) GetProperty(p string) (dbus.Variant, error) { return dbus.Variant{}, nil }
func (n noopBusObject) StoreProperty(p string, value any) error    { return nil }
func (n noopBusObject) SetProperty(p string, v any) error          { return nil }
func (n noopBusObject) Destination() string                        { return "" }
func (n noopBusObject) Path() dbus.ObjectPath                      { return "/" }
func (n noopBusObject) AddMatchSignal(iface, member string, opts ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{}
}
func (n noopBusObject) RemoveMatchSignal(iface, member string, opts ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{}
}

// safeConn wraps fakeDBusConn so BusObject() returns a noop, not nil.
type safeConn struct {
	*fakeDBusConn
}

func (s *safeConn) BusObject() dbus.BusObject { return noopBusObject{} }

func TestMprisPublishesMediaEvent(t *testing.T) {
	b := bus.New()
	gotCh := make(chan state.MediaPlayer, 1)
	b.Subscribe(bus.TopicMedia, func(e bus.Event) {
		gotCh <- e.Data.(state.MediaPlayer)
	})

	fake := newFakeConn()
	svc := mpris.NewWithConn(&safeConn{fake}, b)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.Run(ctx) //nolint:errcheck

	fake.signalCh <- &dbus.Signal{
		Sender: "org.mpris.MediaPlayer2.spotify",
		Name:   "org.freedesktop.DBus.Properties.PropertiesChanged",
		Body: []any{
			"org.mpris.MediaPlayer2.Player",
			map[string]dbus.Variant{
				"PlaybackStatus": dbus.MakeVariant("Playing"),
				"Metadata": dbus.MakeVariant(map[string]dbus.Variant{
					"xesam:title":  dbus.MakeVariant("Bohemian Rhapsody"),
					"xesam:artist": dbus.MakeVariant([]string{"Queen"}),
				}),
				"CanGoNext":     dbus.MakeVariant(true),
				"CanGoPrevious": dbus.MakeVariant(true),
			},
			[]string{},
		},
	}

	select {
	case got := <-gotCh:
		if !got.Playing {
			t.Fatal("expected Playing=true")
		}
		if got.Title != "Bohemian Rhapsody" {
			t.Fatalf("unexpected title: %q", got.Title)
		}
		if got.Artist != "Queen" {
			t.Fatalf("unexpected artist: %q", got.Artist)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for media event")
	}
}
