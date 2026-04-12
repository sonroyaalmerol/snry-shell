package upower_test

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/upower"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

type fakeBusObject struct {
	properties map[string]dbus.Variant
	methods    map[string]any // method name → result to Store
}

var _ dbus.BusObject = (*fakeBusObject)(nil)

func (f *fakeBusObject) Call(method string, flags dbus.Flags, args ...any) *dbus.Call {
	result, ok := f.methods[method]
	if !ok {
		return &dbus.Call{Err: dbus.ErrMsgNoObject}
	}
	return &dbus.Call{Body: []any{result}}
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
	objects map[string]*fakeBusObject
}

func newFakeConn() *fakeDBusConn {
	return &fakeDBusConn{objects: make(map[string]*fakeBusObject)}
}

func (f *fakeDBusConn) Object(dest string, path dbus.ObjectPath) dbus.BusObject {
	key := dest + string(path)
	obj, ok := f.objects[key]
	if !ok {
		return &fakeBusObject{properties: map[string]dbus.Variant{}}
	}
	return obj
}

func (f *fakeDBusConn) Signal(ch chan<- *dbus.Signal)   {}
func (f *fakeDBusConn) RemoveSignal(ch chan<- *dbus.Signal) {}

func (f *fakeDBusConn) BusObject() dbus.BusObject {
	return &fakeBusObject{properties: map[string]dbus.Variant{}}
}

func (f *fakeDBusConn) AddMatchSignal(opts ...dbus.MatchOption) error { return nil }

func batteryKey(path string) string {
	return "org.freedesktop.UPower" + path
}

func TestBatteryCharging(t *testing.T) {
	b := bus.New()
	var got state.BatteryState
	b.Subscribe(bus.TopicBattery, func(e bus.Event) {
		got = e.Data.(state.BatteryState)
	})

	fake := newFakeConn()
	devicePath := "/org/freedesktop/UPower/devices/battery_BAT0"
	fake.objects[batteryKey("/org/freedesktop/UPower")] = &fakeBusObject{
		methods: map[string]any{
			"org.freedesktop.UPower.EnumerateDevices": []dbus.ObjectPath{dbus.ObjectPath(devicePath)},
		},
	}
	fake.objects[batteryKey(devicePath)] = &fakeBusObject{
		properties: map[string]dbus.Variant{
			"org.freedesktop.UPower.Device.Type":       dbus.MakeVariant(uint32(2)),
			"org.freedesktop.UPower.Device.Percentage": dbus.MakeVariant(float64(85)),
			"org.freedesktop.UPower.Device.State":      dbus.MakeVariant(uint32(1)),
		},
	}

	svc := upower.New(fake, b)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	svc.Run(ctx) //nolint:errcheck

	if !got.Present {
		t.Fatal("expected Present=true")
	}
	if got.Percentage != 85 {
		t.Fatalf("expected 85%%, got %f", got.Percentage)
	}
	if !got.Charging {
		t.Fatal("expected Charging=true")
	}
}

func TestBatteryDischarging(t *testing.T) {
	b := bus.New()
	var got state.BatteryState
	b.Subscribe(bus.TopicBattery, func(e bus.Event) {
		got = e.Data.(state.BatteryState)
	})

	fake := newFakeConn()
	devicePath := "/org/freedesktop/UPower/devices/battery_BAT0"
	fake.objects[batteryKey("/org/freedesktop/UPower")] = &fakeBusObject{
		methods: map[string]any{
			"org.freedesktop.UPower.EnumerateDevices": []dbus.ObjectPath{dbus.ObjectPath(devicePath)},
		},
	}
	fake.objects[batteryKey(devicePath)] = &fakeBusObject{
		properties: map[string]dbus.Variant{
			"org.freedesktop.UPower.Device.Type":       dbus.MakeVariant(uint32(2)),
			"org.freedesktop.UPower.Device.Percentage": dbus.MakeVariant(float64(42)),
			"org.freedesktop.UPower.Device.State":      dbus.MakeVariant(uint32(2)), // Discharging
		},
	}

	svc := upower.New(fake, b)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	svc.Run(ctx) //nolint:errcheck

	if got.Charging {
		t.Fatal("expected Charging=false")
	}
	if got.Percentage != 42 {
		t.Fatalf("expected 42%%, got %f", got.Percentage)
	}
}
