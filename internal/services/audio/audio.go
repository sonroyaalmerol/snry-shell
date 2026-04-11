package audio

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jfreymuth/pulse/proto"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

const (
	defaultSink   = "@DEFAULT_SINK@"
	defaultSource = "@DEFAULT_SOURCE@"
)

type Service struct {
	bus        *bus.Bus
	mu         sync.Mutex
	last       state.AudioSink
	volumeStep float64
}

func New(b *bus.Bus) *Service {
	return &Service{bus: b, volumeStep: 0.05}
}

func (s *Service) UpdateStep(step float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.volumeStep = step
}

func (s *Service) VolumeStep() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.volumeStep
}

func NewWithDefaults(b *bus.Bus) *Service {
	return New(b)
}

func (s *Service) Run(ctx context.Context) error {
	for {
		if err := s.run(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("[audio] connection lost: %v, reconnecting in 2s", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *Service) run(ctx context.Context) error {
	client, conn, err := proto.Connect("")
	if err != nil {
		return err
	}
	defer conn.Close()

	updateCh := make(chan struct{}, 8)
	errCh := make(chan error, 1)

	client.Callback = func(val interface{}) {
		switch v := val.(type) {
		case *proto.SubscribeEvent:
			fac := v.Event.GetFacility()
			if fac == proto.EventSink || fac == proto.EventSource {
				select {
				case updateCh <- struct{}{}:
				default:
				}
			}
		case *proto.ConnectionClosed:
			select {
			case errCh <- fmt.Errorf("PA connection closed"):
			default:
			}
		}
	}

	if err := client.Request(&proto.SetClientName{Props: proto.PropList{
		"application.name": proto.PropListString("snry-shell"),
	}}, nil); err != nil {
		return err
	}

	if err := client.Request(&proto.Subscribe{
		Mask: proto.SubscriptionMaskSink | proto.SubscriptionMaskSource,
	}, nil); err != nil {
		return err
	}

	s.poll(client)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		case <-updateCh:
			// Debounce: drain rapid successive events.
			time.Sleep(50 * time.Millisecond)
		drainLoop:
			for {
				select {
				case <-updateCh:
				default:
					break drainLoop
				}
			}
			s.poll(client)
		}
	}
}

func (s *Service) poll(client *proto.Client) {
	sink, err := queryAudio(client)
	if err != nil {
		log.Printf("[audio] query: %v", err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if sink.Volume != s.last.Volume || sink.Muted != s.last.Muted || sink.MicMuted != s.last.MicMuted {
		s.last = sink
		s.bus.Publish(bus.TopicAudio, sink)
	}
}

func queryAudio(client *proto.Client) (state.AudioSink, error) {
	var sinkReply proto.GetSinkInfoReply
	if err := client.Request(&proto.GetSinkInfo{
		SinkIndex: proto.Undefined,
		SinkName:  defaultSink,
	}, &sinkReply); err != nil {
		return state.AudioSink{}, err
	}

	var sourceReply proto.GetSourceInfoReply
	if err := client.Request(&proto.GetSourceInfo{
		SourceIndex: proto.Undefined,
		SourceName:  defaultSource,
	}, &sourceReply); err != nil {
		return state.AudioSink{}, err
	}

	vol := channelVolumesToFloat(sinkReply.ChannelVolumes)
	return state.AudioSink{
		Volume:   vol,
		Muted:    sinkReply.Mute,
		MicMuted: sourceReply.Mute,
	}, nil
}

// channelVolumesToFloat converts PA channel volumes to a 0.0–1.5 float.
func channelVolumesToFloat(cv proto.ChannelVolumes) float64 {
	if len(cv) == 0 {
		return 0
	}
	var acc int64
	for _, v := range cv {
		acc += int64(v)
	}
	acc /= int64(len(cv))
	return float64(acc) / float64(proto.VolumeNorm)
}

// floatToChannelVolumes converts a 0.0–1.5 float to per-channel PA volumes.
func floatToChannelVolumes(v float64, channels int) proto.ChannelVolumes {
	if channels <= 0 {
		channels = 2
	}
	raw := uint32(v * float64(proto.VolumeNorm))
	if raw > uint32(proto.VolumeMax) {
		raw = uint32(proto.VolumeMax)
	}
	vols := make(proto.ChannelVolumes, channels)
	for i := range vols {
		vols[i] = raw
	}
	return vols
}

// withClient opens a short-lived PulseAudio connection and calls fn.
func withClient(fn func(*proto.Client) error) error {
	client, conn, err := proto.Connect("")
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := client.Request(&proto.SetClientName{Props: proto.PropList{
		"application.name": proto.PropListString("snry-shell"),
	}}, nil); err != nil {
		return err
	}
	return fn(client)
}

// SetVolume sets the default sink volume. volume is 0.0–1.5.
func (s *Service) SetVolume(volume float64) error {
	if volume < 0 {
		volume = 0
	}
	if volume > 1.5 {
		volume = 1.5
	}
	err := withClient(func(c *proto.Client) error {
		var reply proto.GetSinkInfoReply
		if err := c.Request(&proto.GetSinkInfo{
			SinkIndex: proto.Undefined,
			SinkName:  defaultSink,
		}, &reply); err != nil {
			return err
		}
		return c.Request(&proto.SetSinkVolume{
			SinkIndex:      proto.Undefined,
			SinkName:       defaultSink,
			ChannelVolumes: floatToChannelVolumes(volume, len(reply.ChannelVolumes)),
		}, nil)
	})
	if err == nil {
		s.triggerPoll()
	}
	return err
}

// AdjustVolume changes the default sink volume by delta, clamped to [0, 1.5].
func (s *Service) AdjustVolume(delta float64) error {
	err := withClient(func(c *proto.Client) error {
		var reply proto.GetSinkInfoReply
		if err := c.Request(&proto.GetSinkInfo{
			SinkIndex: proto.Undefined,
			SinkName:  defaultSink,
		}, &reply); err != nil {
			return err
		}
		next := channelVolumesToFloat(reply.ChannelVolumes) + delta
		if next < 0 {
			next = 0
		}
		if next > 1.5 {
			next = 1.5
		}
		return c.Request(&proto.SetSinkVolume{
			SinkIndex:      proto.Undefined,
			SinkName:       defaultSink,
			ChannelVolumes: floatToChannelVolumes(next, len(reply.ChannelVolumes)),
		}, nil)
	})
	if err == nil {
		s.triggerPoll()
	}
	return err
}

// ToggleMute toggles mute on the default audio sink.
func (s *Service) ToggleMute() error {
	err := withClient(func(c *proto.Client) error {
		var reply proto.GetSinkInfoReply
		if err := c.Request(&proto.GetSinkInfo{
			SinkIndex: proto.Undefined,
			SinkName:  defaultSink,
		}, &reply); err != nil {
			return err
		}
		return c.Request(&proto.SetSinkMute{
			SinkIndex: proto.Undefined,
			SinkName:  defaultSink,
			Mute:      !reply.Mute,
		}, nil)
	})
	if err == nil {
		s.triggerPoll()
	}
	return err
}

// ToggleMicMute toggles mute on the default audio source (microphone).
func (s *Service) ToggleMicMute() error {
	err := withClient(func(c *proto.Client) error {
		var reply proto.GetSourceInfoReply
		if err := c.Request(&proto.GetSourceInfo{
			SourceIndex: proto.Undefined,
			SourceName:  defaultSource,
		}, &reply); err != nil {
			return err
		}
		return c.Request(&proto.SetSourceMute{
			SourceIndex: proto.Undefined,
			SourceName:  defaultSource,
			Mute:        !reply.Mute,
		}, nil)
	})
	if err == nil {
		s.triggerPoll()
	}
	return err
}

func (s *Service) triggerPoll() {
	go withClient(func(c *proto.Client) error {
		s.poll(c)
		return nil
	})
}
