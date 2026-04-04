package network_test

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/network"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// fakeBusObject lets us control property responses.
type fakeBusObject struct {
	properties map[string]dbus.Variant
}

var _ dbus.BusObject = (*fakeBusObject)(nil)

func (f *fakeBusObject) Call(method string, flags dbus.Flags, args ...any) *dbus.Call {
	return &dbus.Call{}
}
func (f *fakeBusObject) CallWithContext(ctx context.Context, method string, flags dbus.Flags, args ...any) *dbus.Call {
	return &dbus.Call{}
}
func (f *fakeBusObject) Go(method string, flags dbus.Flags, ch chan *dbus.Call, args ...any) *dbus.Call {
	return &dbus.Call{}
}
func (f *fakeBusObject) GoWithContext(ctx context.Context, method string, flags dbus.Flags, ch chan *dbus.Call, args ...any) *dbus.Call {
	return &dbus.Call{}
}
func (f *fakeBusObject) GetProperty(prop string) (dbus.Variant, error) {
	v, ok := f.properties[prop]
	if !ok {
		return dbus.Variant{}, dbus.ErrMsgNoObject
	}
	return v, nil
}
func (f *fakeBusObject) StoreProperty(p string, value any) error { return nil }
func (f *fakeBusObject) SetProperty(p string, v any) error       { return nil }
func (f *fakeBusObject) Destination() string                     { return "" }
func (f *fakeBusObject) Path() dbus.ObjectPath                   { return "/" }
func (f *fakeBusObject) AddMatchSignal(iface, member string, opts ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{}
}
func (f *fakeBusObject) RemoveMatchSignal(iface, member string, opts ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{}
}

type fakeDBusConn struct {
	objects  map[string]*fakeBusObject
	signalCh chan *dbus.Signal
}

func newFakeConn() *fakeDBusConn {
	return &fakeDBusConn{
		objects:  make(map[string]*fakeBusObject),
		signalCh: make(chan *dbus.Signal, 8),
	}
}

func (f *fakeDBusConn) Object(dest string, path dbus.ObjectPath) dbus.BusObject {
	key := dest + string(path)
	obj, ok := f.objects[key]
	if !ok {
		return &fakeBusObject{properties: map[string]dbus.Variant{}}
	}
	return obj
}

func (f *fakeDBusConn) Signal(ch chan<- *dbus.Signal) {
	go func() {
		for sig := range f.signalCh {
			ch <- sig
		}
	}()
}

func (f *fakeDBusConn) BusObject() dbus.BusObject {
	return &fakeBusObject{properties: map[string]dbus.Variant{}}
}

func (f *fakeDBusConn) AddMatchSignal(opts ...dbus.MatchOption) error { return nil }

func TestNetworkConnectedState(t *testing.T) {
	b := bus.New()
	var got state.NetworkState
	b.Subscribe(bus.TopicNetwork, func(e bus.Event) {
		got = e.Data.(state.NetworkState)
	})

	fake := newFakeConn()
	nmKey := "org.freedesktop.NetworkManager/org/freedesktop/NetworkManager"
	fake.objects[nmKey] = &fakeBusObject{
		properties: map[string]dbus.Variant{
			"org.freedesktop.NetworkManager.State":          dbus.MakeVariant(uint32(70)),
			"org.freedesktop.NetworkManager.Devices":      dbus.MakeVariant([]dbus.ObjectPath{}),
		},
	}

	svc := network.NewWithConn(fake, b)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	svc.Run(ctx) //nolint:errcheck

	if !got.Connected {
		t.Fatal("expected Connected=true when state=70")
	}
}

func TestNetworkDisconnectedState(t *testing.T) {
	b := bus.New()
	var got state.NetworkState
	b.Subscribe(bus.TopicNetwork, func(e bus.Event) {
		got = e.Data.(state.NetworkState)
	})

	fake := newFakeConn()
	nmKey := "org.freedesktop.NetworkManager/org/freedesktop/NetworkManager"
	fake.objects[nmKey] = &fakeBusObject{
		properties: map[string]dbus.Variant{
			"org.freedesktop.NetworkManager.State":          dbus.MakeVariant(uint32(30)),
			"org.freedesktop.NetworkManager.Devices":      dbus.MakeVariant([]dbus.ObjectPath{}),
		},
	}

	svc := network.NewWithConn(fake, b)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	svc.Run(ctx) //nolint:errcheck

	if got.Connected {
		t.Fatal("expected Connected=false when state=30")
	}
}
