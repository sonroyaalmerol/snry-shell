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

	TopicFloatingImage    Topic = "floatingimage"
	TopicBluetooth        Topic = "bluetooth"
	TopicNightMode        Topic = "nightmode"
	TopicSystemControls   Topic = "systemcontrols"
	TopicSessionAction    Topic = "session"
	TopicScreenLock       Topic = "screenlock"
	TopicResources        Topic = "resources"
	TopicKeyboard         Topic = "keyboard"
	TopicWiFiNetworks     Topic = "wifinetworks"
	TopicBluetoothDevices Topic = "btdevices"
	TopicDND              Topic = "dnd"
	TopicTrayItems        Topic = "trayitems"
	TopicTrayActivate     Topic = "trayactivate"
	TopicTextInputFocus   Topic = "textinputfocus"
	TopicTabletMode       Topic = "tabletmode"
	TopicInputMode        Topic = "inputmode"
	TopicFullscreen       Topic = "fullscreen"
	TopicPopupTrigger     Topic = "popuptrigger"
	TopicStore            Topic = "store"
	TopicThemeChanged     Topic = "themechanged"
	TopicSettingsChanged  Topic = "settingschanged"
	TopicDarkMode         Topic = "darkmode"
	TopicNetworkManager   Topic = "networkmanager"
	TopicIdleInhibit      Topic = "idleinhibit"
	TopicOSKState         Topic = "oskstate"
)

type Event struct {
	Topic Topic
	Data  any
}

type Handler func(Event)

type Publisher interface{ Publish(topic Topic, data any) }

type UnsubscribeFunc func()

type entry struct {
	id uint64
	h  Handler
}

type Bus struct {
	mu     sync.Mutex
	subs   map[Topic][]entry
	last   map[Topic]Event
	nextID uint64
}

func New() *Bus {
	return &Bus{
		subs: make(map[Topic][]entry),
		last: make(map[Topic]Event),
	}
}

func (b *Bus) Subscribe(topic Topic, h Handler) UnsubscribeFunc {
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	b.subs[topic] = append(b.subs[topic], entry{id: id, h: h})
	ev, ok := b.last[topic]
	b.mu.Unlock()

	if ok {
		h(ev)
	}

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		entries := b.subs[topic]
		for i, e := range entries {
			if e.id == id {
				b.subs[topic] = append(entries[:i], entries[i+1:]...)
				return
			}
		}
	}
}

func (b *Bus) Publish(topic Topic, data any) {
	b.mu.Lock()
	ev := Event{Topic: topic, Data: data}
	b.last[topic] = ev
	entries := make([]entry, len(b.subs[topic]))
	copy(entries, b.subs[topic])
	b.mu.Unlock()

	for _, e := range entries {
		e.h(ev)
	}
}
