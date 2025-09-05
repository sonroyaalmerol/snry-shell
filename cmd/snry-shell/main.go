package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/sonroyaalmerol/snry-shell/surfaces"
)

func main() {
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "--toggle-") {
		sendControl(os.Args[1])
		return
	}
	os.Exit(surfaces.Run())
}

func sendControl(arg string) {
	conn, err := net.Dial("unix", "/tmp/snry-shell.sock")
	if err != nil {
		fmt.Fprintln(os.Stderr, "snry-shell not running")
		os.Exit(1)
	}
	defer conn.Close()
	// Strip leading "--" from the arg.
	cmd := arg[2:] // "toggle-overview" etc.
	if _, err := conn.Write([]byte(cmd)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
