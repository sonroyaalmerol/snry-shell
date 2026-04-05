package bus

import "sync"

type Topic string

const (
	TopicWorkspaces   Topic = "workspaces"
	TopicActiveWindow Topic = "activewindow"
	TopicAudio        Topic = "audio"
	TopicBattery      Topic = "battery"
	TopicNetwork      Topic = "network"
	TopicNotification Topic = "notification"
	TopicMedia        Topic = "media"
	TopicMediaTick    Topic = "mediatick"
	TopicBrightness   Topic = "brightness"
	TopicClipboard    Topic = "clipboard"

	TopicFloatingImage  Topic = "floatingimage"
	TopicBluetooth      Topic = "bluetooth"
	TopicNightMode      Topic = "nightmode"
	TopicSystemControls Topic = "systemcontrols"
	TopicSessionAction  Topic = "session"
	TopicScreenLock     Topic = "screenlock"
	TopicResources        Topic = "resources"
	TopicKeyboard         Topic = "keyboard"
	TopicAudioMixer       Topic = "audiomixer"
	TopicWiFiNetworks     Topic = "wifinetworks"
	TopicBluetoothDevices Topic = "btdevices"
	TopicPomodoro         Topic = "pomodoro"
	TopicTodo             Topic = "todo"
	TopicDND              Topic = "dnd"
	TopicTrayItems        Topic = "trayitems"
	TopicTrayActivate     Topic = "trayactivate"
	TopicTextInputFocus   Topic = "textinputfocus"
)

type Event struct { Topic Topic; Data any }
type Handler func(Event)
type Publisher interface { Publish(topic Topic, data any) }

type Bus struct {
	mu       sync.RWMutex
	handlers map[Topic][]Handler
	last     map[Topic]Event // last published event per topic for replay
}

func New() *Bus { return &Bus{handlers: make(map[Topic][]Handler), last: make(map[Topic]Event)} }
func (b *Bus) Subscribe(topic Topic, h Handler) {
	b.mu.Lock(); defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], h)
	// Replay last event so late subscribers get current state.
	if ev, ok := b.last[topic]; ok {
		h(ev)
	}
}
func (b *Bus) Publish(topic Topic, data any) {
	b.mu.Lock()
	ev := Event{Topic: topic, Data: data}
	b.last[topic] = ev
	handlers := make([]Handler, len(b.handlers[topic]))
	copy(handlers, b.handlers[topic])
	b.mu.Unlock()
	for _, h := range handlers { h(ev) }
}
