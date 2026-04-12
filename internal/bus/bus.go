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
	subs   *xsync.Map[Topic, *topicBucket]
	last   *xsync.Map[Topic, Event]
	nextID atomic.Uint64
}

// topicBucket groups a topic's subscriber list. xsync.Map values must be
// pointer-sized, so we box the slice in a struct updated via Compute.
type topicBucket struct {
	entries []entry
}

func New() *Bus {
	return &Bus{
		subs: xsync.NewMap[Topic, *topicBucket](),
		last: xsync.NewMap[Topic, Event](),
	}
}

func (b *Bus) Subscribe(topic Topic, h Handler) UnsubscribeFunc {
	id := b.nextID.Add(1)

	b.subs.Compute(topic, func(old *topicBucket, loaded bool) (*topicBucket, xsync.ComputeOp) {
		if !loaded || old == nil {
			return &topicBucket{entries: []entry{{id: id, h: h}}}, xsync.UpdateOp
		}
		old.entries = append(old.entries, entry{id: id, h: h})
		return old, xsync.UpdateOp
	})

	if ev, ok := b.last.Load(topic); ok {
		h(ev)
	}

	return func() {
		b.subs.Compute(topic, func(old *topicBucket, loaded bool) (*topicBucket, xsync.ComputeOp) {
			if !loaded || old == nil {
				return nil, xsync.DeleteOp
			}
			for i, e := range old.entries {
				if e.id == id {
					old.entries = append(old.entries[:i], old.entries[i+1:]...)
					if len(old.entries) == 0 {
						return nil, xsync.DeleteOp
					}
					return old, xsync.UpdateOp
				}
			}
			return old, xsync.CancelOp
		})
	}
}

func (b *Bus) Publish(topic Topic, data any) {
	ev := Event{Topic: topic, Data: data}
	b.last.Store(topic, ev)

	bucket, ok := b.subs.Load(topic)
	if !ok || bucket == nil {
		return
	}
	// Snapshot the entries slice. The Compute callback mutates entries
	// in-place (append), so we copy for a consistent iteration view.
	entries := make([]entry, len(bucket.entries))
	copy(entries, bucket.entries)

	for _, e := range entries {
		e.h(ev)
	}
}
