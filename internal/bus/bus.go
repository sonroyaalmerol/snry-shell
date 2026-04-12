package bus

import (
	"sync/atomic"

	"github.com/puzpuzpuz/xsync/v4"
)

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
	subs   *xsync.Map[Topic, []entry]
	last   *xsync.Map[Topic, Event]
	nextID atomic.Uint64
}

func New() *Bus {
	return &Bus{
		subs: xsync.NewMap[Topic, []entry](),
		last: xsync.NewMap[Topic, Event](),
	}
}

func (b *Bus) Subscribe(topic Topic, h Handler) UnsubscribeFunc {
	id := b.nextID.Add(1)

	b.subs.Compute(topic, func(old []entry, loaded bool) ([]entry, xsync.ComputeOp) {
		return append(old, entry{id: id, h: h}), xsync.UpdateOp
	})

	if ev, ok := b.last.Load(topic); ok {
		h(ev)
	}

	return func() {
		b.subs.Compute(topic, func(old []entry, loaded bool) ([]entry, xsync.ComputeOp) {
			if !loaded {
				return nil, xsync.DeleteOp
			}
			for i, e := range old {
				if e.id == id {
					return append(old[:i:i], old[i+1:]...), xsync.UpdateOp
				}
			}
			return old, xsync.CancelOp
		})
	}
}

func (b *Bus) Publish(topic Topic, data any) {
	ev := Event{Topic: topic, Data: data}
	b.last.Store(topic, ev)

	entries, ok := b.subs.Load(topic)
	if !ok {
		return
	}

	for _, e := range entries {
		e.h(ev)
	}
}
