package surfaces

import (
	"os"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func Run() int {
	app := gtk.NewApplication("sh.snry.shell", 0)
	app.ConnectActivate(func() {})
	return app.Run(os.Args)
}
