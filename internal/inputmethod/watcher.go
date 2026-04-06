package inputmethod

import (
	"context"
	"log"

	"github.com/rajveermalviya/go-wayland/wayland/client"
	protocol "github.com/sonroyaalmerol/snry-shell/internal/inputmethod/protocol"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
)

// Watcher monitors zwp_input_method_v2 activate/deactivate events on the
// Wayland display and publishes TopicTextInputFocus events to the bus.
// Returns nil, nil if the protocol is not available.
type Watcher struct {
	display *client.Display
	bus     *bus.Bus
}

const (
	imInterfaceName = "zwp_input_method_manager_v2"
	seatInterface   = "wl_seat"
	seatVersion     = 8
	imVersion       = 1
)

// New connects to the Wayland display, binds to zwp_input_method_manager_v2
// and wl_seat, creates an input method, and sets up event handlers.
// Returns nil, nil if the protocol is not available.
func New(b *bus.Bus) (*Watcher, error) {
	display, err := client.Connect("")
	if err != nil {
		log.Printf("[IM] cannot connect to Wayland display: %v", err)
		return nil, nil
	}

	registry, err := display.GetRegistry()
	if err != nil {
		log.Printf("[IM] cannot get registry: %v", err)
		display.Destroy()
		return nil, nil
	}

	var (
		imManagerName uint32
		imManagerVer  uint32
		seatName      uint32
		seatVer       uint32
		found         int
	)

	registry.SetGlobalHandler(func(e client.RegistryGlobalEvent) {
		switch e.Interface {
		case imInterfaceName:
			imManagerName = e.Name
			imManagerVer = e.Version
			found++
		case seatInterface:
			seatName = e.Name
			seatVer = e.Version
			found++
		}
	})

	// Round-trip to receive global events.
	if err := roundtrip(display); err != nil {
		log.Printf("[IM] registry round-trip failed: %v", err)
		display.Destroy()
		return nil, nil
	}

	if found < 2 {
		if imManagerName == 0 {
			log.Printf("[IM] %s not advertised by compositor", imInterfaceName)
		}
		if seatName == 0 {
			log.Printf("[IM] %s not advertised by compositor", seatInterface)
		}
		display.Destroy()
		return nil, nil
	}

	// Bind the input method manager and seat.
	manager := protocol.NewInputMethodManager(display.Context())
	if err := registry.Bind(imManagerName, imInterfaceName, min(imManagerVer, imVersion), manager); err != nil {
		log.Printf("[IM] bind %s failed: %v", imInterfaceName, err)
		display.Destroy()
		return nil, nil
	}

	seat := client.NewSeat(display.Context())
	if err := registry.Bind(seatName, seatInterface, min(seatVer, seatVersion), seat); err != nil {
		log.Printf("[IM] bind %s failed: %v", seatInterface, err)
		display.Destroy()
		return nil, nil
	}

	im, err := manager.GetInputMethod(seat)
	if err != nil {
		log.Printf("[IM] GetInputMethod failed: %v", err)
		display.Destroy()
		return nil, nil
	}

	w := &Watcher{display: display, bus: b}

	im.SetActivateHandler(func(protocol.InputMethodActivateEvent) {
		log.Printf("[IM] activate")
		b.Publish(bus.TopicTextInputFocus, true)
	})

	im.SetDeactivateHandler(func(protocol.InputMethodDeactivateEvent) {
		log.Printf("[IM] deactivate")
		b.Publish(bus.TopicTextInputFocus, false)
	})

	im.SetUnavailableHandler(func(protocol.InputMethodUnavailableEvent) {
		log.Printf("[IM] unavailable")
		b.Publish(bus.TopicTextInputFocus, false)
	})

	im.SetDoneHandler(func(protocol.InputMethodDoneEvent) {
		// State sync point — no action needed for focus detection.
	})

	im.SetContentTypeHandler(func(protocol.InputMethodContentTypeEvent) {
		// Future: adjust keyboard layout based on hint/purpose (e.g. number pad).
	})

	log.Printf("[IM] connected to input-method-v2 protocol")
	return w, nil
}

// Run dispatches Wayland events in a loop. Blocks until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	log.Printf("[IM] watching for input-method events")

	// Close the connection from a separate goroutine to unblock
	// the blocking Dispatch() call when context is cancelled.
	go func() {
		<-ctx.Done()
		w.display.Context().Close()
	}()

	defer w.display.Context().Close()

	for {
		if err := w.display.Context().Dispatch(); err != nil {
			log.Printf("[IM] dispatch ended: %v", err)
			return
		}
	}
}

// roundtrip performs a sync round-trip to ensure all pending events are
// delivered before returning.
func roundtrip(display *client.Display) error {
	cb, err := display.Sync()
	if err != nil {
		return err
	}
	done := make(chan struct{})
	cb.SetDoneHandler(func(client.CallbackDoneEvent) {
		close(done)
	})
	for {
		if err := display.Context().Dispatch(); err != nil {
			return err
		}
		select {
		case <-done:
			return nil
		default:
		}
	}
}
