package state

import "time"

type Workspace struct {
	ID       int
	Name     string
	Active   bool
	Occupied bool
	Classes  []string
}
type ActiveWindow struct {
	Class string
	Title string
}
type AudioSink struct {
	Name   string
	Volume float64
	Muted  bool
}
type BatteryState struct {
	Percentage float64
	Charging   bool
	Present    bool
}
type NetworkState struct {
	SSID            string
	Connected       bool
	Strength        int
	WirelessEnabled bool
}
type Notification struct {
	ID      uint32
	AppName string
	Summary string
	Body    string
	Urgency byte
	Timeout int32
}
type MediaPlayer struct {
	PlayerName string
	Title      string
	Artist     string
	ArtPath    string
	Playing    bool
	CanNext    bool
	CanPrev    bool
	Position   float64
	Duration   float64
}
type ClipboardEntry struct {
	ID      int
	Preview string
}
type BrightnessState struct {
	Current int
	Max     int
}

type SystemControls struct {
	Volume           float64
	Brightness       float64
	NetworkEnabled   bool
	BluetoothEnabled bool
	NightModeEnabled bool
}
type BluetoothState struct {
	Powered    bool
	Connected  bool
	DeviceName string
}
type SessionAction int

const (
	SessionLock SessionAction = iota
	SessionSuspend
	SessionReboot
	SessionShutdown
	SessionLogout
)

type LockScreenState struct{ Locked bool }
type MediaTick struct {
	PlayerName string
	Position   float64
	Duration   float64
	At         time.Time
}

type ResourceState struct {
	CPU float64
	RAM float64
}
type AudioApp struct {
	Name   string
	ID     int
	Volume float64
	Muted  bool
}
type AudioMixerState struct{ Apps []AudioApp }
type WiFiNetwork struct {
	SSID      string
	Signal    int
	Security  string
	Connected bool
	Saved     bool
}
type BluetoothDevice struct {
	Address   string
	Name      string
	Paired    bool
	Connected bool
	Icon      string
	Trusted   bool
}

// NMConnection represents a NetworkManager connection profile
type NMConnection struct {
	Path           string
	Name           string
	UUID           string
	Type           string
	TypeLabel      string
	SSID           string
	MAC            string
	VPNType        string
	Autoconnect    bool
	Secured        bool
	IPv4Method     string
	IPv6Method     string
	IPv4Configured bool
	IPv6Configured bool
	Active         bool
}

// NMDevice represents a NetworkManager device
type NMDevice struct {
	Path             string
	Interface        string
	Type             uint32
	State            uint32
	HwAddress        string
	ActiveConnection string
	HasIP4           bool
	HasIP6           bool
}
type PomodoroState struct {
	Phase             string
	Remaining         time.Duration
	Running           bool
	SessionsCompleted int
}
type TodoItem struct {
	ID   int
	Text string
	Done bool
}

type ColorScheme struct {
	Primary, OnPrimary, PrimaryContainer, OnPrimaryContainer                             string
	Secondary, OnSecondary, SecondaryContainer, OnSecondaryContainer                     string
	Tertiary, OnTertiary, TertiaryContainer, OnTertiaryContainer                         string
	Error, OnError, ErrorContainer, OnErrorContainer                                     string
	Surface, SurfaceDim, SurfaceBright                                                   string
	SurfaceContainer, SurfaceContainerLow, SurfaceContainerHigh, SurfaceContainerHighest string
	OnSurface, OnSurfaceVariant                                                          string
	Background, OnBackground                                                             string
	Outline, OutlineVariant                                                              string
	Subtext                                                                              string
}
