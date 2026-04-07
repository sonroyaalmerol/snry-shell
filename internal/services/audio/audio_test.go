package audio

import (
	"testing"

	"github.com/jfreymuth/pulse/proto"
)

func TestChannelVolumesToFloat(t *testing.T) {
	tests := []struct {
		volumes []uint32
		want    float64
	}{
		{[]uint32{uint32(proto.VolumeNorm)}, 1.0},
		{[]uint32{0}, 0.0},
		{[]uint32{uint32(proto.VolumeNorm / 2), uint32(proto.VolumeNorm / 2)}, 0.5},
	}
	for _, tt := range tests {
		got := channelVolumesToFloat(proto.ChannelVolumes(tt.volumes))
		if got != tt.want {
			t.Errorf("channelVolumesToFloat(%v) = %f, want %f", tt.volumes, got, tt.want)
		}
	}
}

func TestFloatToChannelVolumes(t *testing.T) {
	vols := floatToChannelVolumes(0.75, 2)
	if len(vols) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(vols))
	}
	want := uint32(0.75 * float64(proto.VolumeNorm))
	if vols[0] != want || vols[1] != want {
		t.Errorf("floatToChannelVolumes(0.75, 2) = %v, want [%d %d]", vols, want, want)
	}
}

func TestFloatToChannelVolumesClamp(t *testing.T) {
	vols := floatToChannelVolumes(2.0, 1)
	if vols[0] > uint32(proto.VolumeMax) {
		t.Errorf("expected clamped to VolumeMax, got %d", vols[0])
	}
}
