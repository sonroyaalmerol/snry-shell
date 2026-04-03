// Package controlsocket provides a Unix domain socket server that receives
// toggle commands from snry-shell CLI invocations and publishes them on the bus.
package controlsocket

import (
	"net"
	"os"
	"strings"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
)

const socketPath = "/tmp/snry-shell.sock"

// Start creates a Unix domain socket listener and dispatches incoming
// commands to bus.TopicSystemControls. It runs in the background.
func Start(b *bus.Bus) error {
	os.Remove(socketPath)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			go handleConn(conn, b)
		}
	}()
	return nil
}

func handleConn(conn net.Conn, b *bus.Bus) {
	defer conn.Close()

	buf := make([]byte, 256)
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
