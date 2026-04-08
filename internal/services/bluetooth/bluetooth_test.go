package bluetooth_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/bluetooth"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// fakeBusObject satisfies dbus.BusObject for tests.
type fakeBusObject struct {
	mu           sync.Mutex
	props        map[string]dbus.Variant
	setPropErr   error // if non-nil, SetProperty returns this error
	setPropCalls atomic.Int32
}

var _ dbus.BusObject = (*fakeBusObject)(nil)

func (f *fakeBusObject) Call(string, dbus.Flags, ...any) *dbus.Call { return &dbus.Call{} }
func (f *fakeBusObject) CallWithContext(context.Context, string, dbus.Flags, ...any) *dbus.Call {
	return &dbus.Call{}
}
func (f *fakeBusObject) Go(string, dbus.Flags, chan *dbus.Call, ...any) *dbus.Call {
	return &dbus.Call{}
}
func (f *fakeBusObject) GoWithContext(context.Context, string, dbus.Flags, chan *dbus.Call, ...any) *dbus.Call {
	return &dbus.Call{}
}
func (f *fakeBusObject) GetProperty(p string) (dbus.Variant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.props[p]
	if !ok {
		return dbus.Variant{}, dbus.ErrMsgNoObject
	}
	return v, nil
}
func (f *fakeBusObject) StoreProperty(string, any) error { return nil }
func (f *fakeBusObject) SetProperty(p string, v any) error {
	f.setPropCalls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setPropErr != nil {
		return f.setPropErr
	}
	// Extract the actual variant value.
	if vv, ok := v.(dbus.Variant); ok {
		f.props[p] = vv
	}
	return nil
}
func (f *fakeBusObject) Destination() string   { return "" }
func (f *fakeBusObject) Path() dbus.ObjectPath { return "/" }
func (f *fakeBusObject) AddMatchSignal(string, string, ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{}
}
func (f *fakeBusObject) RemoveMatchSignal(string, string, ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{}
}

type fakeDBusConn struct {
	obj *fakeBusObject
}

func (f *fakeDBusConn) Object(string, dbus.ObjectPath) dbus.BusObject { return f.obj }
func (f *fakeDBusConn) Signal(chan<- *dbus.Signal)                    {}
func (f *fakeDBusConn) RemoveSignal(chan<- *dbus.Signal)              {}
func (f *fakeDBusConn) AddMatchSignal(...dbus.MatchOption) error      { return nil }
func (f *fakeDBusConn) BusObject() dbus.BusObject                     { return f.obj }

func TestBluetoothPollPublishes(t *testing.T) {
	b := bus.New()
	gotCh := make(chan state.BluetoothState, 1)
	b.Subscribe(bus.TopicBluetooth, func(e bus.Event) {
		gotCh <- e.Data.(state.BluetoothState)
	})

	conn := &fakeDBusConn{obj: &fakeBusObject{
		props: map[string]dbus.Variant{
			"org.bluez.Adapter1.Powered": dbus.MakeVariant(true),
		},
	}}

	svc := bluetooth.NewWithConn(conn, b)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()     // immediate cancel — poll runs once then exits
	svc.Run(ctx) //nolint:errcheck

	select {
	case got := <-gotCh:
		if !got.Powered {
			t.Fatal("expected Powered=true")
		}
	default:
		t.Fatal("expected a bluetooth event")
	}
}

func TestBluetoothPollOff(t *testing.T) {
	b := bus.New()
	gotCh := make(chan state.BluetoothState, 1)
	b.Subscribe(bus.TopicBluetooth, func(e bus.Event) {
		gotCh <- e.Data.(state.BluetoothState)
	})

	conn := &fakeDBusConn{obj: &fakeBusObject{
		props: map[string]dbus.Variant{
			"org.bluez.Adapter1.Powered": dbus.MakeVariant(false),
		},
	}}

	svc := bluetooth.NewWithConn(conn, b)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.Run(ctx) //nolint:errcheck

	select {
	case got := <-gotCh:
		if got.Powered {
			t.Fatal("expected Powered=false")
		}
	default:
		t.Fatal("expected a bluetooth event")
	}
}

// TestSetPoweredChangesState verifies that SetPowered(false) actually changes
// the adapter property and poll() reflects the new state.
func TestSetPoweredChangesState(t *testing.T) {
	b := bus.New()
	gotCh := make(chan state.BluetoothState, 4)
	b.Subscribe(bus.TopicBluetooth, func(e bus.Event) {
		gotCh <- e.Data.(state.BluetoothState)
	})

	obj := &fakeBusObject{
		props: map[string]dbus.Variant{
			"org.bluez.Adapter1.Powered": dbus.MakeVariant(true),
		},
	}
	conn := &fakeDBusConn{obj: obj}

	svc := bluetooth.NewWithConn(conn, b)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the service Run loop (it will poll once then block on signals).
	go svc.Run(ctx)

	// Drain the initial poll event.
	select {
	case got := <-gotCh:
		if !got.Powered {
			t.Fatal("initial poll should be Powered=true")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for initial poll")
	}

	// User clicks toggle off.
	err := svc.SetPowered(false)
	if err != nil {
		t.Fatalf("SetPowered(false) failed: %v", err)
	}

	// SetPowered should have called SetProperty. Verify the prop actually changed.
	got, err := obj.GetProperty("org.bluez.Adapter1.Powered")
	if err != nil {
		t.Fatalf("GetProperty failed: %v", err)
	}
	powered, _ := got.Value().(bool)
	if powered {
		t.Fatal("expected Powered=false after SetPowered(false)")
	}

	// State changed via SetProperty, but no signal-based re-poll happens
	// in tests (no D-Bus signals). The toggle should still work because
	// SetPowered succeeded and the bus replay will serve the correct state.
	select {
	case got := <-gotCh:
		if got.Powered {
			t.Fatal("no re-poll expected after successful SetPowered(false)")
		}
	default:
		// OK — no stale re-poll (the old explicit re-poll was removed).
	}
}

// TestSetPoweredFailedRePublishesActualState verifies that when SetProperty
// fails, SetPowered re-polls and publishes the actual (unchanged) state so
// the UI correctly reflects that bluetooth is still on.
func TestSetPoweredFailedRePublishesActualState(t *testing.T) {
	b := bus.New()
	gotCh := make(chan state.BluetoothState, 4)
	b.Subscribe(bus.TopicBluetooth, func(e bus.Event) {
		gotCh <- e.Data.(state.BluetoothState)
	})

	obj := &fakeBusObject{
		props: map[string]dbus.Variant{
			"org.bluez.Adapter1.Powered": dbus.MakeVariant(true),
		},
		setPropErr: errors.New("org.freedesktop.DBus.Error.PropertyUpdate: Operation not permitted"),
	}
	conn := &fakeDBusConn{obj: obj}

	svc := bluetooth.NewWithConn(conn, b)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go svc.Run(ctx)

	// Drain initial poll.
	select {
	case got := <-gotCh:
		if !got.Powered {
			t.Fatal("initial poll should be Powered=true")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for initial poll")
	}

	// Clear the channel before the action.
	for len(gotCh) > 0 {
		<-gotCh
	}

	// User clicks toggle off — SetProperty FAILS.
	err := svc.SetPowered(false)
	if err == nil {
		t.Fatal("expected SetPowered to fail")
	}

	// The error re-poll should have published Powered:true (the actual state).
	select {
	case got := <-gotCh:
		if !got.Powered {
			t.Fatal("after failed SetPowered(false), re-poll should publish Powered=true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for error re-poll")
	}
}

// TestConcurrentPollSafety verifies that poll() can be called concurrently
// without data races (detected by go test -race).
func TestConcurrentPollSafety(t *testing.T) {
	b := bus.New()
	obj := &fakeBusObject{
		props: map[string]dbus.Variant{
			"org.bluez.Adapter1.Powered": dbus.MakeVariant(true),
		},
	}
	conn := &fakeDBusConn{obj: obj}
	svc := bluetooth.NewWithConn(conn, b)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// poll() is unexported, but SetPowered triggers a poll after delay.
			// Instead, we test by calling GetDevices which also does D-Bus calls.
			_, _ = svc.GetDevices()
		}()
	}
	wg.Wait()
}
