package dbusutil

import "github.com/godbus/dbus/v5"

type DBusConn interface {
	Object(dest string, path dbus.ObjectPath) dbus.BusObject
	Signal(ch chan<- *dbus.Signal)
	BusObject() dbus.BusObject
	AddMatchSignal(opts ...dbus.MatchOption) error
}

type RealConn struct {
	Conn *dbus.Conn
}

func (r *RealConn) Object(dest string, path dbus.ObjectPath) dbus.BusObject {
	return r.Conn.Object(dest, path)
}

func (r *RealConn) Signal(ch chan<- *dbus.Signal) {
	r.Conn.Signal(ch)
}

func (r *RealConn) BusObject() dbus.BusObject {
	return r.Conn.BusObject()
}

func (r *RealConn) AddMatchSignal(opts ...dbus.MatchOption) error {
	return r.Conn.AddMatchSignal(opts...)
}

func NewRealConn(conn *dbus.Conn) *RealConn {
	return &RealConn{Conn: conn}
}
