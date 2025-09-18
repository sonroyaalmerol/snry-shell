package audio_test

import (
	"context"
	"testing"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/audio"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

func TestParseWpctlVolume(t *testing.T) {
	tests := []struct {
		input string
		vol   float64
		muted bool
	}{
		{"Volume: 0.75", 0.75, false},
		{"Volume: 0.40 [MUTED]", 0.40, true},
		{"Volume: 1.00", 1.00, false},
		{"Volume: 0.00 [MUTED]", 0.00, true},
	}
	for _, tt := range tests {
		got, err := audio.ParseWpctlVolume(tt.input)
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", tt.input, err)
		}
		if got.Volume != tt.vol {
			t.Fatalf("input %q: expected volume %f, got %f", tt.input, tt.vol, got.Volume)
		}
		if got.Muted != tt.muted {
			t.Fatalf("input %q: expected muted %v, got %v", tt.input, tt.muted, got.Muted)
		}
	}
}

func TestParseWpctlVolumeInvalid(t *testing.T) {
	_, err := audio.ParseWpctlVolume("garbage")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

type fakeRunner struct {
	output string
}

func (f *fakeRunner) Output(args ...string) ([]byte, error) {
	return []byte(f.output), nil
}

func (f *fakeRunner) Run(args ...string) error { return nil }

func TestServicePublishesAudioEvent(t *testing.T) {
	b := bus.New()
	var got state.AudioSink
	b.Subscribe(bus.TopicAudio, func(e bus.Event) {
		got = e.Data.(state.AudioSink)
	})

	runner := &fakeRunner{output: "Volume: 0.60"}
	svc := audio.New(runner, b)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	svc.Run(ctx) //nolint:errcheck

	if got.Volume != 0.60 {
		t.Fatalf("expected 0.60, got %f", got.Volume)
	}
}
