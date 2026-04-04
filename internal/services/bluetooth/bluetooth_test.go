package bluetooth_test

import (
	"context"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/bluetooth"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// fakeBusObject satisfies dbus.BusObject for tests.
type fakeBusObject struct {
	props map[string]dbus.Variant
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
	v, ok := f.props[p]
	if !ok {
		return dbus.Variant{}, dbus.ErrMsgNoObject
	}
	return v, nil
}
func (f *fakeBusObject) StoreProperty(string, any) error   { return nil }
func (f *fakeBusObject) SetProperty(p string, v any) error { return nil }
func (f *fakeBusObject) Destination() string               { return "" }
func (f *fakeBusObject) Path() dbus.ObjectPath             { return "/" }
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
