package servicerefs

import (
	"github.com/sonroyaalmerol/snry-shell/internal/services/audio"
	"github.com/sonroyaalmerol/snry-shell/internal/services/brightness"
)

type ServiceRefs struct {
	Audio      *audio.Service
	Brightness *brightness.Service
}
