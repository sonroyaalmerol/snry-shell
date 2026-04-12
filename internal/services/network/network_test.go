package network_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/services/network"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// --- fake D-Bus infrastructure ---

type fakeBusObject struct {
	properties map[string]dbus.Variant
	callResult map[string]any // method -> result to store
	callErr    map[string]error
}

var _ dbus.BusObject = (*fakeBusObject)(nil)

func (f *fakeBusObject) Call(method string, flags dbus.Flags, args ...any) *dbus.Call {
	err := f.callErr[method]
	var store any
	if s, ok := f.callResult[method]; ok {
		store = s
	}
	return &dbus.Call{Err: err, Body: []any{store}}
}
func (f *fakeBusObject) CallWithContext(ctx context.Context, method string, flags dbus.Flags, args ...any) *dbus.Call {
	return f.Call(method, flags, args...)
}
func (f *fakeBusObject) Go(method string, flags dbus.Flags, ch chan *dbus.Call, args ...any) *dbus.Call {
	return &dbus.Call{}
}
func (f *fakeBusObject) GoWithContext(ctx context.Context, method string, flags dbus.Flags, ch chan *dbus.Call, args ...any) *dbus.Call {
	return &dbus.Call{}
}
func (f *fakeBusObject) GetProperty(prop string) (dbus.Variant, error) {
	v, ok := f.properties[prop]
	if !ok {
		return dbus.Variant{}, dbus.ErrMsgNoObject
	}
	return v, nil
}
func (f *fakeBusObject) StoreProperty(p string, value any) error { return nil }
func (f *fakeBusObject) SetProperty(p string, v any) error       { return nil }
func (f *fakeBusObject) Destination() string                     { return "" }
func (f *fakeBusObject) Path() dbus.ObjectPath                   { return "/" }
func (f *fakeBusObject) AddMatchSignal(iface, member string, opts ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{}
}
func (f *fakeBusObject) RemoveMatchSignal(iface, member string, opts ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{}
}

type fakeDBusConn struct {
	mu       sync.Mutex
	objects  map[string]*fakeBusObject
	signalCh chan *dbus.Signal
}

func newFakeConn() *fakeDBusConn {
	return &fakeDBusConn{
		objects:  make(map[string]*fakeBusObject),
		signalCh: make(chan *dbus.Signal, 32),
	}
}

func (f *fakeDBusConn) setObject(dest string, path dbus.ObjectPath, obj *fakeBusObject) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[dest+string(path)] = obj
}

func (f *fakeDBusConn) Object(dest string, path dbus.ObjectPath) dbus.BusObject {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := dest + string(path)
	obj, ok := f.objects[key]
	if !ok {
		return &fakeBusObject{properties: map[string]dbus.Variant{}}
	}
	return obj
}

func (f *fakeDBusConn) Signal(ch chan<- *dbus.Signal) {
	go func() {
		for sig := range f.signalCh {
			ch <- sig
		}
	}()
}
func (f *fakeDBusConn) RemoveSignal(ch chan<- *dbus.Signal) {}
func (f *fakeDBusConn) BusObject() dbus.BusObject {
	return &fakeBusObject{properties: map[string]dbus.Variant{}}
}
func (f *fakeDBusConn) AddMatchSignal(opts ...dbus.MatchOption) error { return nil }

// --- helpers ---

func nmObj() *fakeBusObject {
	return &fakeBusObject{
		properties: map[string]dbus.Variant{
			"org.freedesktop.NetworkManager.State":            dbus.MakeVariant(uint32(70)),
			"org.freedesktop.NetworkManager.PrimaryConnection": dbus.MakeVariant(dbus.ObjectPath("/org/freedesktop/NetworkManager/ActiveConnection/1")),
			"org.freedesktop.NetworkManager.Devices":          dbus.MakeVariant([]dbus.ObjectPath{"/org/freedesktop/NetworkManager/Devices/1"}),
			"org.freedesktop.NetworkManager.WirelessEnabled":  dbus.MakeVariant(true),
		},
	}
}

func wifiDeviceObj(ssid string, apPaths []dbus.ObjectPath) *fakeBusObject {
	props := map[string]dbus.Variant{
		"org.freedesktop.NetworkManager.Device.DeviceType":    dbus.MakeVariant(uint32(2)), // WiFi
		"org.freedesktop.NetworkManager.Device.ActiveConnection": dbus.MakeVariant(dbus.ObjectPath("/org/freedesktop/NetworkManager/ActiveConnection/1")),
		"org.freedesktop.NetworkManager.Device.Wireless.AccessPoints": dbus.MakeVariant(apPaths),
	}
	if ssid != "" {
		props["org.freedesktop.NetworkManager.Device.Wireless.ActiveAccessPoint"] = dbus.MakeVariant(apPaths[0])
	} else {
		props["org.freedesktop.NetworkManager.Device.Wireless.ActiveAccessPoint"] = dbus.MakeVariant(dbus.ObjectPath("/"))
	}
	return &fakeBusObject{properties: props}
}

func apObj(ssid string, strength uint8, flags uint32) *fakeBusObject {
	return &fakeBusObject{
		properties: map[string]dbus.Variant{
			"org.freedesktop.NetworkManager.AccessPoint.Ssid":    dbus.MakeVariant([]byte(ssid)),
			"org.freedesktop.NetworkManager.AccessPoint.Strength": dbus.MakeVariant(strength),
			"org.freedesktop.NetworkManager.AccessPoint.WpaFlags": dbus.MakeVariant(flags),
			"org.freedesktop.NetworkManager.AccessPoint.RsnFlags": dbus.MakeVariant(flags),
		},
	}
}

func activeConnObj(settingsPath dbus.ObjectPath) *fakeBusObject {
	return &fakeBusObject{
		properties: map[string]dbus.Variant{
			"org.freedesktop.NetworkManager.Connection.Active.Connection": dbus.MakeVariant(settingsPath),
		},
	}
}

func settingsObj(ssid string) *fakeBusObject {
	return &fakeBusObject{
		callResult: map[string]any{
			"org.freedesktop.NetworkManager.Settings.Connection.GetSettings": map[string]map[string]dbus.Variant{
				"connection": {
					"id":   dbus.MakeVariant(ssid),
					"type": dbus.MakeVariant("802-11-wireless"),
				},
				"802-11-wireless": {
					"ssid": dbus.MakeVariant([]byte(ssid)),
				},
			},
		},
	}
}

func setupService(fake *fakeDBusConn, b *bus.Bus) *network.Service {
	svc := network.New(fake, b)

	// Set up NM object
	fake.setObject("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager", nmObj())

	// Set up WiFi device
	apPaths := []dbus.ObjectPath{
		"/org/freedesktop/NetworkManager/AccessPoint/1",
		"/org/freedesktop/NetworkManager/AccessPoint/2",
	}
	fake.setObject("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager/Devices/1", wifiDeviceObj("HomeWiFi", apPaths))

	// Set up APs
	fake.setObject("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager/AccessPoint/1", apObj("HomeWiFi", 80, 0))
	fake.setObject("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager/AccessPoint/2", apObj("GuestWiFi", 40, 1))

	// Set up active connection
	fake.setObject("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager/ActiveConnection/1",
		activeConnObj("/org/freedesktop/NetworkManager/Settings/Connection/1"))
	fake.setObject("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager/Settings/Connection/1",
		settingsObj("HomeWiFi"))

	return svc
}

// --- Tests ---

// TestWiFiListPersistsAcrossSignalBursts verifies that a burst of D-Bus signals
// after ScanWiFi does not wipe the WiFi list. This reproduces the "list disappears
// after a few seconds" bug.
func TestWiFiListPersistsAcrossSignalBursts(t *testing.T) {
	b := bus.New()
	fake := newFakeConn()
	svc := setupService(fake, b)

	var states []state.NetworkState
	var mu sync.Mutex
	b.Subscribe(bus.TopicNetwork, func(e bus.Event) {
		ns, ok := e.Data.(state.NetworkState)
		if !ok {
			return
		}
		mu.Lock()
		states = append(states, ns)
		mu.Unlock()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go svc.Run(ctx)

	// Wait for initial Run() publish.
	time.Sleep(100 * time.Millisecond)

	// Simulate user opening popup and scanning.
	if err := svc.ScanWiFi(); err != nil {
		t.Fatalf("ScanWiFi failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify scan published WiFi networks.
	mu.Lock()
	lastState := states[len(states)-1]
	mu.Unlock()
	if len(lastState.WiFiNetworks) == 0 {
		t.Fatal("ScanWiFi should have published WiFi networks")
	}
	t.Logf("ScanWiFi published %d networks: %+v", len(lastState.WiFiNetworks), lastState.WiFiNetworks)

	// Simulate NM signal burst (like signal strength updates).
	// These should NOT wipe the WiFi list.
	for i := 0; i < 5; i++ {
		fake.signalCh <- &dbus.Signal{
			Path: "/org/freedesktop/NetworkManager/AccessPoint/1",
		}
	}

	// Wait for debounce to fire and settle.
	time.Sleep(600 * time.Millisecond)

	// Check that the last published state still has WiFi networks.
	mu.Lock()
	lastState = states[len(states)-1]
	mu.Unlock()
	if len(lastState.WiFiNetworks) == 0 {
		t.Errorf("WiFi networks disappeared after signal burst! Last state: %+v", lastState)
		t.Errorf("Total states published: %d", len(states))
		for i, s := range states {
			t.Logf("  state[%d]: SSID=%q Connected=%v WiFiNetworks=%d", i, s.SSID, s.Connected, len(s.WiFiNetworks))
		}
	}
}

// TestWiFiListDoesNotFlickerOnSameState verifies that when connection state hasn't
// actually changed, the WiFi networks list stays populated. This catches the case
// where fetchState returns transient empty SSID during NM transitions.
func TestWiFiListDoesNotFlickerOnSameState(t *testing.T) {
	b := bus.New()
	fake := newFakeConn()
	svc := setupService(fake, b)

	var publishCount int
	var mu sync.Mutex
	var lastNetworks []state.WiFiNetwork
	b.Subscribe(bus.TopicNetwork, func(e bus.Event) {
		ns, ok := e.Data.(state.NetworkState)
		if !ok {
			return
		}
		mu.Lock()
		publishCount++
		lastNetworks = ns.WiFiNetworks
		mu.Unlock()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go svc.Run(ctx)

	time.Sleep(100 * time.Millisecond)

	// Scan to populate the list.
	svc.ScanWiFi()
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	countAfterScan := publishCount
	networksAfterScan := len(lastNetworks)
	mu.Unlock()

	if networksAfterScan == 0 {
		t.Fatal("Expected WiFi networks after scan")
	}

	// Now change NM state to return empty SSID (simulating a brief disconnect).
	nmKey := "org.freedesktop.NetworkManager" + "/org/freedesktop/NetworkManager"
	fake.setObject("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager", &fakeBusObject{
		properties: map[string]dbus.Variant{
			"org.freedesktop.NetworkManager.State":             dbus.MakeVariant(uint32(40)), // connecting
			"org.freedesktop.NetworkManager.PrimaryConnection": dbus.MakeVariant(dbus.ObjectPath("/")),
			"org.freedesktop.NetworkManager.Devices":           dbus.MakeVariant([]dbus.ObjectPath{"/org/freedesktop/NetworkManager/Devices/1"}),
			"org.freedesktop.NetworkManager.WirelessEnabled":   dbus.MakeVariant(true),
		},
	})
	// Also clear the device's active AP
	fake.setObject("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager/Devices/1", wifiDeviceObj("", []dbus.ObjectPath{
		"/org/freedesktop/NetworkManager/AccessPoint/1",
		"/org/freedesktop/NetworkManager/AccessPoint/2",
	}))

	// Fire signal to trigger monitorSignals.
	// Override the object key that was already used.
	_ = nmKey
	fake.signalCh <- &dbus.Signal{Path: "/org/freedesktop/NetworkManager/Devices/1"}
	fake.signalCh <- &dbus.Signal{Path: "/org/freedesktop/NetworkManager/Devices/1"}

	// Wait for debounce.
	time.Sleep(600 * time.Millisecond)

	mu.Lock()
	finalNetworks := len(lastNetworks)
	totalPublishes := publishCount
	mu.Unlock()

	t.Logf("Publishes: afterScan=%d total=%d", countAfterScan, totalPublishes)
	t.Logf("Networks: afterScan=%d final=%d", networksAfterScan, finalNetworks)

	// The list should still have networks even though SSID went empty temporarily.
	if finalNetworks == 0 {
		t.Error("WiFi networks list was wiped by transient empty-SSID state")
	}
}

// TestCachedWiFiStamping verifies that fetchFullState correctly stamps Connected
// based on current SSID without re-scanning APs.
func TestCachedWiFiStamping(t *testing.T) {
	b := bus.New()
	fake := newFakeConn()
	svc := setupService(fake, b)

	var lastState state.NetworkState
	b.Subscribe(bus.TopicNetwork, func(e bus.Event) {
		ns, ok := e.Data.(state.NetworkState)
		if !ok {
			return
		}
		lastState = ns
	})

	// Directly test the caching mechanism via ScanWiFi.
	svc.ScanWiFi()

	// Verify networks are present and correct Connected flag.
	if len(lastState.WiFiNetworks) == 0 {
		t.Fatal("Expected WiFi networks after scan")
	}

	// HomeWiFi should be Connected (it's the active SSID).
	found := false
	for _, net := range lastState.WiFiNetworks {
		if net.SSID == "HomeWiFi" {
			found = true
			if !net.Connected {
				t.Error("HomeWiFi should be Connected (matches active SSID)")
			}
		}
		if net.SSID == "GuestWiFi" {
			if net.Connected {
				t.Error("GuestWiFi should NOT be Connected")
			}
		}
	}
	if !found {
		t.Error("HomeWiFi not found in scan results")
	}
}

// TestWiFiListPersistsAfterScanThenSignals reproduces the real-world flow:
//  1. Run() publishes initial state (empty WiFiNetworks, cachedWiFiLoaded=false)
//  2. User opens popup → ScanWiFi() populates list
//  3. NM sends routine signals (strength updates, not connection changes)
//  4. The list must NOT disappear
//
// This catches the bug where monitorSignals publishes a state that wipes the
// WiFi list because the state diff doesn't account for cached WiFi networks.
func TestWiFiListPersistsAfterScanThenSignals(t *testing.T) {
	b := bus.New()
	fake := newFakeConn()
	svc := setupService(fake, b)

	var states []state.NetworkState
	var mu sync.Mutex
	b.Subscribe(bus.TopicNetwork, func(e bus.Event) {
		ns, ok := e.Data.(state.NetworkState)
		if !ok {
			return
		}
		mu.Lock()
		states = append(states, ns)
		mu.Unlock()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Step 1: Start Run() — publishes initial state with empty WiFiNetworks.
	go svc.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	initialState := states[len(states)-1]
	mu.Unlock()
	if len(initialState.WiFiNetworks) != 0 {
		t.Log("Initial state unexpectedly has WiFi networks (ok, not the issue)")
	}
	t.Logf("Initial publish: SSID=%q Connected=%v WiFiNetworks=%d", initialState.SSID, initialState.Connected, len(initialState.WiFiNetworks))

	// Step 2: User opens popup → ScanWiFi fires.
	if err := svc.ScanWiFi(); err != nil {
		t.Fatalf("ScanWiFi failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	afterScan := states[len(states)-1]
	mu.Unlock()
	if len(afterScan.WiFiNetworks) == 0 {
		t.Fatal("ScanWiFi should have published WiFi networks")
	}
	t.Logf("After scan: SSID=%q Connected=%v WiFiNetworks=%d", afterScan.SSID, afterScan.Connected, len(afterScan.WiFiNetworks))

	// Step 3: NM sends routine signals (e.g., strength updates).
	// These do NOT change SSID or Connected state.
	for i := 0; i < 10; i++ {
		fake.signalCh <- &dbus.Signal{
			Path: "/org/freedesktop/NetworkManager/AccessPoint/1",
		}
	}

	// Wait for debounce to fire and settle.
	time.Sleep(800 * time.Millisecond)

	// Step 4: Check the LAST published state still has WiFi networks.
	mu.Lock()
	lastState := states[len(states)-1]
	totalStates := len(states)
	mu.Unlock()

	t.Logf("Final state: SSID=%q Connected=%v WiFiNetworks=%d", lastState.SSID, lastState.Connected, len(lastState.WiFiNetworks))
	t.Logf("Total publishes: %d", totalStates)
	for i, s := range states {
		t.Logf("  state[%d]: SSID=%q Connected=%v WiFiNetworks=%d", i, s.SSID, s.Connected, len(s.WiFiNetworks))
	}

	if len(lastState.WiFiNetworks) == 0 {
		t.Error("WiFi networks disappeared after routine NM signals!")
	}
}

// TestSignalPublishAlwaysIncludesCachedWiFi verifies that any publish from
// monitorSignals includes cached WiFi networks once a scan has been done.
func TestSignalPublishAlwaysIncludesCachedWiFi(t *testing.T) {
	b := bus.New()
	fake := newFakeConn()
	svc := setupService(fake, b)

	var states []state.NetworkState
	var mu sync.Mutex
	b.Subscribe(bus.TopicNetwork, func(e bus.Event) {
		ns, ok := e.Data.(state.NetworkState)
		if !ok {
			return
		}
		mu.Lock()
		states = append(states, ns)
		mu.Unlock()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go svc.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	// Scan to populate cache.
	svc.ScanWiFi()
	time.Sleep(100 * time.Millisecond)

	// Now change NM state slightly (e.g., signal strength change doesn't
	// affect SSID or Connected, but NM sends PropertiesChanged).
	// Update the AP strength to simulate a real signal update.
	fake.setObject("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager/AccessPoint/1",
		apObj("HomeWiFi", 90, 0)) // strength changed from 80 to 90

	// Send signal to trigger monitorSignals.
	fake.signalCh <- &dbus.Signal{
		Path: "/org/freedesktop/NetworkManager/AccessPoint/1",
	}

	// Wait for debounce.
	time.Sleep(600 * time.Millisecond)

	// Every published state after the scan should have WiFi networks.
	mu.Lock()
	defer mu.Unlock()
	for i, s := range states {
		if len(s.WiFiNetworks) == 0 && i > 0 { // skip initial Run() publish
			t.Errorf("state[%d] has empty WiFiNetworks after scan was done", i)
		}
	}
}
