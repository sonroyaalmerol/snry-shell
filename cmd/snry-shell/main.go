package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/sonroyaalmerol/snry-shell/internal/controlsocket"
	"github.com/sonroyaalmerol/snry-shell/surfaces"
	"github.com/sonroyaalmerol/snry-shell/surfaces/controlpanel"
)

func main() {
	if len(os.Args) > 1 {
		switch {
		case strings.HasPrefix(os.Args[1], "--toggle-"):
			sendControl(os.Args[1])
			return
		case os.Args[1] == "--control-panel" || os.Args[1] == "-c":
			os.Exit(controlpanel.Run())
			return
		}
	}
	os.Exit(surfaces.Run())
}

func sendControl(arg string) {
	conn, err := net.Dial("unix", controlsocket.DefaultPath)
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
