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
	TopicTheme        Topic = "theme"
	TopicBrightness   Topic = "brightness"
	TopicClipboard    Topic = "clipboard"

	TopicFloatingImage  Topic = "floatingimage"
	TopicBluetooth      Topic = "bluetooth"
	TopicNightMode      Topic = "nightmode"
	TopicSystemControls Topic = "systemcontrols"
	TopicWallpapers     Topic = "wallpapers"
	TopicSessionAction  Topic = "session"
	TopicScreenLock     Topic = "screenlock"
	TopicSettings       Topic = "settings"

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
)

type Event struct { Topic Topic; Data any }
type Handler func(Event)
type Publisher interface { Publish(topic Topic, data any) }

type Bus struct {
	mu       sync.RWMutex
	handlers map[Topic][]Handler
}

func New() *Bus { return &Bus{handlers: make(map[Topic][]Handler)} }
func (b *Bus) Subscribe(topic Topic, h Handler) {
	b.mu.Lock(); defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], h)
}
func (b *Bus) Publish(topic Topic, data any) {
	b.mu.RLock(); defer b.mu.RUnlock()
	ev := Event{Topic: topic, Data: data}
	for _, h := range b.handlers[topic] { h(ev) }
}
