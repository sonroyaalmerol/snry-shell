package inputmethod

import (
	"context"
	"log"

	"github.com/rajveermalviya/go-wayland/wayland/client"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	protocol "github.com/sonroyaalmerol/snry-shell/internal/inputmethod/protocol"
	"github.com/sonroyaalmerol/snry-shell/internal/waylandutil"
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
		log.Printf("[inputmethod] cannot connect to Wayland display: %v", err)
		return nil, nil
	}

	registry, err := display.GetRegistry()
	if err != nil {
		log.Printf("[inputmethod] cannot get registry: %v", err)
		display.Destroy()
		return nil, nil
	}

	var (
		imManagerName uint32
		imManagerVer  uint32
		seatName      uint32
		seatVer       uint32
	)

	registry.SetGlobalHandler(func(e client.RegistryGlobalEvent) {
		switch e.Interface {
		case imInterfaceName:
			imManagerName = e.Name
			imManagerVer = e.Version
		case seatInterface:
			if seatName == 0 {
				seatName = e.Name
				seatVer = e.Version
			}
		}
	})

	// Round-trip to receive global events.
	if err := waylandutil.Roundtrip(display); err != nil {
		log.Printf("[inputmethod] registry round-trip failed: %v", err)
		display.Destroy()
		return nil, nil
	}

	if imManagerName == 0 || seatName == 0 {
		if imManagerName == 0 {
			log.Printf("[inputmethod] %s not advertised by compositor", imInterfaceName)
		}
		if seatName == 0 {
			log.Printf("[inputmethod] %s not advertised by compositor", seatInterface)
		}
		display.Destroy()
		return nil, nil
	}

	// Bind the input method manager and seat using the FixedBind workaround.
	manager := protocol.NewInputMethodManager(display.Context())
	if err := waylandutil.FixedBind(registry, imManagerName, imInterfaceName, min(imManagerVer, imVersion), manager); err != nil {
		log.Printf("[inputmethod] bind %s failed: %v", imInterfaceName, err)
		display.Destroy()
		return nil, nil
	}

	seat := client.NewSeat(display.Context())
	if err := waylandutil.FixedBind(registry, seatName, seatInterface, min(seatVer, seatVersion), seat); err != nil {
		log.Printf("[inputmethod] bind %s failed: %v", seatInterface, err)
		display.Destroy()
		return nil, nil
	}

	// Wait for Binds to process.
	if err := waylandutil.Roundtrip(display); err != nil {
		log.Printf("[inputmethod] bind round-trip failed: %v", err)
		display.Destroy()
		return nil, nil
	}

	im, err := manager.GetInputMethod(seat)
	if err != nil {
		log.Printf("[inputmethod] GetInputMethod failed: %v", err)
		display.Destroy()
		return nil, nil
	}

	w := &Watcher{display: display, bus: b}

	im.SetActivateHandler(func(protocol.InputMethodActivateEvent) {
		log.Printf("[inputmethod] activate")
		b.Publish(bus.TopicTextInputFocus, true)
	})

	im.SetDeactivateHandler(func(protocol.InputMethodDeactivateEvent) {
		log.Printf("[inputmethod] deactivate")
		b.Publish(bus.TopicTextInputFocus, false)
	})

	im.SetUnavailableHandler(func(protocol.InputMethodUnavailableEvent) {
		log.Printf("[inputmethod] unavailable")
		b.Publish(bus.TopicTextInputFocus, false)
	})

	im.SetDoneHandler(func(protocol.InputMethodDoneEvent) {
		// State sync point — no action needed for focus detection.
	})

	im.SetContentTypeHandler(func(protocol.InputMethodContentTypeEvent) {
		// Future: adjust keyboard layout based on hint/purpose (e.g. number pad).
	})

	log.Printf("[inputmethod] connected to input-method-v2 protocol")
	return w, nil
}

// Run dispatches Wayland events in a loop. Blocks until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	log.Printf("[inputmethod] watching for input-method events")

	// Capture display for shutdown goroutine.
	d := w.display
	go func() {
		<-ctx.Done()
		d.Context().Close()
	}()

	defer d.Context().Close()

	for {
		if err := d.Context().Dispatch(); err != nil {
			log.Printf("[inputmethod] dispatch ended: %v", err)
			return
		}
	}
}
