package servicerefs

import (
	"github.com/sonroyaalmerol/snry-shell/internal/services/audio"
	"github.com/sonroyaalmerol/snry-shell/internal/services/audiomixer"
	"github.com/sonroyaalmerol/snry-shell/internal/services/bluetooth"
	"github.com/sonroyaalmerol/snry-shell/internal/services/brightness"
	"github.com/sonroyaalmerol/snry-shell/internal/services/clipboard"
	"github.com/sonroyaalmerol/snry-shell/internal/services/darkmode"
	"github.com/sonroyaalmerol/snry-shell/internal/services/hyprland"
	"github.com/sonroyaalmerol/snry-shell/internal/services/inputmode"
	"github.com/sonroyaalmerol/snry-shell/internal/services/mpris"
	"github.com/sonroyaalmerol/snry-shell/internal/services/network"
	"github.com/sonroyaalmerol/snry-shell/internal/services/nightmode"
	"github.com/sonroyaalmerol/snry-shell/internal/services/pomodoro"
	"github.com/sonroyaalmerol/snry-shell/internal/services/resources"
	"github.com/sonroyaalmerol/snry-shell/internal/services/sni"
	"github.com/sonroyaalmerol/snry-shell/internal/services/todo"
)

type ServiceRefs struct {
	Audio      *audio.Service
	Brightness *brightness.Service
	Mpris      *mpris.Service
	Bluetooth  *bluetooth.Service
	Network    *network.Service
	NightMode  *nightmode.Service
	Resources  *resources.Service
	AudioMixer *audiomixer.Service
	Hyprland   *hyprland.Querier
	Pomodoro   *pomodoro.Service
	Todo       *todo.Service
	SNI        *sni.Service
	InputMode  *inputmode.Service
	Clipboard  *clipboard.Service
	DarkMode   *darkmode.Service
}
