package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"

	"github.com/sonroyaalmerol/snry-shell/internal/controlsocket"
	"github.com/sonroyaalmerol/snry-shell/surfaces"
	"github.com/sonroyaalmerol/snry-shell/surfaces/controlpanel"
)

func init() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
}

func main() {
	if len(os.Args) > 1 {
		switch {
		case os.Args[1] == "--control-panel" || os.Args[1] == "-c":
			os.Exit(controlpanel.Run())
			return
		case strings.HasPrefix(os.Args[1], "--"):
			sendControl(os.Args[1])
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
