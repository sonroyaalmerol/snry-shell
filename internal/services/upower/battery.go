package upower

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/dbusutil"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const (
	upowerDest          = "org.freedesktop.UPower"
	upowerPath          = "/org/freedesktop/UPower"
	upowerIface         = "org.freedesktop.UPower"
	upowerDeviceIface   = "org.freedesktop.UPower.Device"
	upowerTypeBattery   = uint32(2)
	upowerStateCharging = uint32(1)
)

type Service struct {
	conn dbusutil.DBusConn
	bus  *bus.Bus
}

func New(conn dbusutil.DBusConn, b *bus.Bus) *Service {
	return &Service{conn: conn, bus: b}
}

func NewWithDefaults(b *bus.Bus) *Service {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return &Service{bus: b}
	}
	return &Service{conn: dbusutil.NewRealConn(conn), bus: b}
}

func (s *Service) Run(ctx context.Context) error {
	if s.conn == nil {
		<-ctx.Done()
		return ctx.Err()
	}

	ch := make(chan *dbus.Signal, 16)
	s.conn.Signal(ch)
	defer s.conn.RemoveSignal(ch)

	s.conn.BusObject().Call(
		"org.freedesktop.DBus.AddMatch", 0,
		fmt.Sprintf("type='signal',sender='%s',interface='%s',member='PropertiesChanged'",
			upowerDest, "org.freedesktop.DBus.Properties"),
	)

	s.query()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-ch:
			if !ok {
				return nil
			}
			s.query()
		}
	}
}

func (s *Service) query() {
	bs, err := s.fetchState()
	if err != nil {
		return
	}
	s.bus.Publish(bus.TopicBattery, bs)
}

func (s *Service) fetchState() (state.BatteryState, error) {
	upObj := s.conn.Object(upowerDest, upowerPath)

	var paths []dbus.ObjectPath
	if err := upObj.Call(upowerIface+".EnumerateDevices", 0).Store(&paths); err != nil {
		// Fallback: use the display device.
		return s.fetchDisplayDevice()
	}

	for _, p := range paths {
		if bs, ok := s.fetchBatteryDevice(p); ok {
			return bs, nil
		}
	}
	return state.BatteryState{}, fmt.Errorf("no battery found")
}

func (s *Service) fetchDisplayDevice() (state.BatteryState, error) {
	displayPath := dbus.ObjectPath(upowerPath + "/devices/DisplayDevice")
	bs, ok := s.fetchBatteryDevice(displayPath)
	if !ok {
		return state.BatteryState{}, fmt.Errorf("display device unavailable")
	}
	return bs, nil
}

func (s *Service) fetchBatteryDevice(path dbus.ObjectPath) (state.BatteryState, bool) {
	obj := s.conn.Object(upowerDest, path)

	typeV, err := obj.GetProperty(upowerDeviceIface + ".Type")
	if err != nil {
		return state.BatteryState{}, false
	}
	if t, ok := typeV.Value().(uint32); !ok || t != upowerTypeBattery {
		return state.BatteryState{}, false
	}

	pctV, err := obj.GetProperty(upowerDeviceIface + ".Percentage")
	if err != nil {
		return state.BatteryState{}, false
	}
	pct, _ := pctV.Value().(float64)

	stateV, err := obj.GetProperty(upowerDeviceIface + ".State")
	if err != nil {
		return state.BatteryState{}, false
	}
	upState, _ := stateV.Value().(uint32)

	return state.BatteryState{
		Percentage: pct,
		Charging:   upState == upowerStateCharging,
		Present:    true,
	}, true
}
