package bus_test

import (
	"testing"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

func TestBusPublishSubscribe(t *testing.T) {
	b := bus.New()
	received := make(chan bus.Event, 1)
	b.Subscribe(bus.TopicAudio, func(e bus.Event) {
		received <- e
	})
	b.Publish(bus.TopicAudio, state.AudioSink{Volume: 0.75})
	ev := <-received
	sink := ev.Data.(state.AudioSink)
	if sink.Volume != 0.75 {
		t.Fatalf("expected 0.75, got %f", sink.Volume)
	}
}

func TestBusMultipleSubscribers(t *testing.T) {
	b := bus.New()
	count := 0
	for range 3 {
		b.Subscribe(bus.TopicBattery, func(e bus.Event) {
			count++
		})
	}
	b.Publish(bus.TopicBattery, state.BatteryState{Percentage: 80})
	if count != 3 {
		t.Fatalf("expected 3 handlers called, got %d", count)
	}
}

func TestBusNoSubscribers(t *testing.T) {
	b := bus.New()
	// Should not panic when no subscribers exist.
	b.Publish(bus.TopicNetwork, state.NetworkState{Connected: true})
}

func TestBusTopicIsolation(t *testing.T) {
	b := bus.New()
	called := false
	b.Subscribe(bus.TopicAudio, func(e bus.Event) {
		called = true
	})
	b.Publish(bus.TopicBattery, state.BatteryState{})
	if called {
		t.Fatal("audio subscriber should not be called for battery topic")
	}
}

func TestBusPublisherInterface(t *testing.T) {
	b := bus.New()
	var pub bus.Publisher = b
	received := make(chan bus.Event, 1)
	b.Subscribe(bus.TopicTheme, func(e bus.Event) {
		received <- e
	})
	pub.Publish(bus.TopicTheme, state.ColorScheme{Primary: "#6750A4"})
	ev := <-received
	scheme := ev.Data.(state.ColorScheme)
	if scheme.Primary != "#6750A4" {
		t.Fatalf("unexpected primary: %q", scheme.Primary)
	}
}

func TestBusReplayOnSubscribe(t *testing.T) {
	b := bus.New()

	// Publish before any subscriber.
	b.Publish(bus.TopicAudio, state.AudioSink{Volume: 0.5})

	// Late subscriber should receive the last published event.
	received := make(chan state.AudioSink, 1)
	b.Subscribe(bus.TopicAudio, func(e bus.Event) {
		received <- e.Data.(state.AudioSink)
	})
	select {
	case sink := <-received:
		if sink.Volume != 0.5 {
			t.Fatalf("replay: expected 0.5, got %f", sink.Volume)
		}
	default:
		t.Fatal("late subscriber should have received replayed event")
	}

	// Replay should give the latest value.
	b.Publish(bus.TopicAudio, state.AudioSink{Volume: 0.9})
	received2 := make(chan state.AudioSink, 1)
	b.Subscribe(bus.TopicAudio, func(e bus.Event) {
		received2 <- e.Data.(state.AudioSink)
	})
	select {
	case sink := <-received2:
		if sink.Volume != 0.9 {
			t.Fatalf("replay latest: expected 0.9, got %f", sink.Volume)
		}
	default:
		t.Fatal("late subscriber should have received latest replayed event")
	}
}

func TestBusNoReplayForUnpublishedTopic(t *testing.T) {
	b := bus.New()
	called := false
	b.Subscribe(bus.TopicClipboard, func(e bus.Event) {
		called = true
	})
	if called {
		t.Fatal("subscriber should not be called for topic with no published events")
	}
}
