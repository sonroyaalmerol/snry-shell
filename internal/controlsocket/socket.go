// Package controlsocket provides a Unix domain socket server that receives
// toggle commands from snry-shell CLI invocations and publishes them on the bus.
package controlsocket

import (
	"net"
	"os"
	"strings"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
)

const DefaultPath = "/tmp/snry-shell.sock"

// Start creates a Unix domain socket listener on the default path and
// dispatches incoming commands to bus.TopicSystemControls. It runs in the
// background. Call Close when the application exits to clean up the socket file.
func Start(b *bus.Bus) (*net.UnixListener, error) {
	return StartAt(b, DefaultPath)
}

// StartAt is like Start but uses the given socket path.
func StartAt(b *bus.Bus, path string) (*net.UnixListener, error) {
	os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}

	go accept(ln, b)
	return ln.(*net.UnixListener), nil
}

func accept(ln net.Listener, b *bus.Bus) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go handleConn(conn, b)
	}
}

func handleConn(conn net.Conn, b *bus.Bus) {
	defer conn.Close()

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	cmd := strings.TrimSpace(string(buf[:n]))
	if cmd == "" {
		return
	}

	b.Publish(bus.TopicSystemControls, cmd)
}

// Close shuts down the listener and removes the socket file.
func Close(ln *net.UnixListener) {
	if addr := ln.Addr(); addr != nil {
		os.Remove(addr.String())
	}
	ln.Close()
}
