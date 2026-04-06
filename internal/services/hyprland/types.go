package hyprland

type HyprWorkspace struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type HyprClient struct {
	Class    string `json:"class"`
	Title    string `json:"title"`
	Address  string `json:"address"`
	At       [2]int `json:"at"`
	Size     [2]int `json:"size"`
	Workspace struct {
		ID int `json:"id"`
	} `json:"workspace"`
	PID     int    `json:"pid"`
	Monitor int    `json:"monitor"`
}

type HyprMonitor struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	Scale           float64 `json:"scale"`
	X               int    `json:"x"`
	Y               int    `json:"y"`
	ActiveWorkspace struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"activeWorkspace"`
	RefreshRate int `json:"refreshRate"`
}

type HyprDevices struct {
	Mice     []HyprMouse     `json:"mice"`
	Keyboard []HyprKeyboard  `json:"keyboards"`
}

type HyprMouse struct {
	Name string `json:"name"`
}

type HyprKeyboard struct {
	Name         string `json:"name"`
	ActiveKeymap string `json:"active_keymap"`
	Main         bool   `json:"main"`
}
