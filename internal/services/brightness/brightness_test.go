package brightness_test

import (
	"testing"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/brightness"
)

func TestNewWithDefaults(t *testing.T) {
	svc := brightness.NewWithDefaults(bus.New())
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}
