package state

import "time"

type Workspace struct { ID int; Name string; Active bool; Occupied bool }
type ActiveWindow struct { Class string; Title string }
type AudioSink struct { Name string; Volume float64; Muted bool }
type BatteryState struct { Percentage float64; Charging bool; Present bool }
type NetworkState struct { SSID string; Connected bool; Strength int; WirelessEnabled bool }
type Notification struct { ID uint32; AppName string; Summary string; Body string; Urgency byte; Timeout int32 }
type MediaPlayer struct { PlayerName string; Title string; Artist string; ArtPath string; Playing bool; CanNext bool; CanPrev bool; Position float64; Duration float64 }
type ClipboardEntry struct { ID int; Preview string }
type BrightnessState struct { Current int; Max int }
