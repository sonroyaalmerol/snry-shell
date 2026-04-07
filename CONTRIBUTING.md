# Contributing to snry-shell

## Development Setup

### System dependencies

**Arch Linux:**
```sh
sudo pacman -S go gtk4 gtk4-layer-shell pkgconf
yay -S swww cliphist hyprpicker
sudo pacman -S wireplumber grim wl-clipboard wf-recorder
```

**Fedora:**
```sh
sudo dnf copr enable solopash/hyprland
sudo dnf install go gtk4-devel gtk4-layer-shell-devel pkgconf gcc
```

**Debian / Ubuntu:**
```sh
sudo apt install golang libgtk-4-dev libgtk4-layer-shell-dev pkg-config gcc
```

The shell requires `CGO_ENABLED=1` (the default). Pure-Go cross-compilation is not possible due to GTK4 and gtk4-layer-shell C bindings.

### Fonts

Icons are rendered as text via the Material Symbols variable font. Install:
- [Google Sans Flex](https://fonts.google.com/specimen/Google+Sans+Flex)
- [Material Symbols Rounded](https://fonts.google.com/specimen/Material+Symbols+Rounded)
- [JetBrains Mono NF](https://www.nerdfonts.com/font-downloads)

### Build & run

```sh
make build
./snry-shell
```

To run with verbose GTK debug output:
```sh
GTK_DEBUG=interactive ./snry-shell
```

### Test

```sh
make test
```

Hardware-optional tests (DDC over I2C) can be enabled:
```sh
DDC_INTEGRATION=1 go test ./internal/ddc/...
```

### Lint

```sh
make vet   # go vet
make fmt   # gofmt -w
```

---

## Architecture Overview

```
cmd/snry-shell/main.go     CLI: flags dispatch to surfaces.Run() or controlpanel.Run()
surfaces/app.go            Orchestrator: wires all services and surfaces, runs GTK main loop
surfaces/*/                One package per UI surface (layer-shell window)
internal/bus/              Pub/sub event bus — the only coupling between services and surfaces
internal/state/            Plain data types for every published event payload
internal/services/*/       One package per backend service
internal/store/            Persistent key-value store (~/.config/snry-shell/store.json)
internal/settings/         Typed shell config (thin wrapper over store)
internal/layershell/       CGo bindings for gtk4-layer-shell
internal/surfaceutil/      Helpers shared by all surfaces (popup panels, toggles, geometry)
internal/gtkutil/          GTK widget helpers (M3 dialogs, buttons, sliders)
internal/theme/            Built-in wallpaper color extraction and CSS generation
internal/uinput/           Virtual keyboard via /dev/uinput (zero-dependency key injection)
internal/ddc/              Pure-Go DDC/CI brightness control (I2C ioctls)
assets/style.css           Base Material Design 3 stylesheet (embedded into the binary)
```

### Data flow

```
Service  ──publish──▶  bus.Bus  ──replay + broadcast──▶  Surface
                                                           │
                                                    glib.IdleAdd
                                                           │
                                                    GTK widget update
```

Services **never** import surface packages. Surfaces **never** call service methods directly except via `ServiceRefs` passed at construction. All other communication is through the bus.

---

## The Event Bus (`internal/bus`)

`bus.Bus` is a synchronous pub/sub broker. Every topic has **replay**: the last published event is stored and delivered immediately to any late subscriber. This means a surface that subscribes after a service has already published will still receive the current state.

```go
// Publish a state change from a service goroutine.
b.Publish(bus.TopicAudio, state.AudioState{Volume: 0.75, Muted: false})

// Subscribe from the GTK main thread (or any goroutine).
b.Subscribe(bus.TopicAudio, func(e bus.Event) {
    st := e.Data.(state.AudioState)
    // If the service already published before this Subscribe call,
    // this handler is called immediately with the last value.
    glib.IdleAdd(func() { label.SetText(fmt.Sprintf("%.0f%%", st.Volume*100)) })
})
```

**Rules:**
- `Publish` is safe to call from any goroutine.
- `Subscribe` handlers are called on the publisher's goroutine — always wrap GTK calls in `glib.IdleAdd`.
- Add new topic constants to `internal/bus/bus.go`.
- Add the matching payload type to `internal/state/state.go`.

---

## Service Patterns (`internal/services/`)

### Interface injection for testability

Every service that touches the OS abstracts its dependencies behind interfaces defined in `internal/services/runner`:

```go
// Runner — one-shot subprocess (e.g. wpctl, hyprctl)
type Runner interface {
    Output(args ...string) ([]byte, error)
    Run(args ...string) error
}

// StreamReader — long-running subprocess with stdout line stream (e.g. pactl subscribe)
type StreamReader interface {
    Stream(args ...string) (io.ReadCloser, error)
}

// Commander — hyprctl-specific command runner
type Commander interface {
    Run(args ...string) ([]byte, error)
}

// PollLoop — shared polling helper used by brightness, resources, darkmode, etc.
func PollLoop(ctx context.Context, interval time.Duration, poll func()) error
```

Use the real implementations in `surfaces/app.go` and fake implementations in tests:

```go
// Production
svc := audio.New(runner.New(), runner.NewStreamReader(), b)

// Test
type fakeRunner struct{ out []byte }
func (f fakeRunner) Output(...string) ([]byte, error) { return f.out, nil }
func (f fakeRunner) Run(...string) error              { return nil }
svc := audio.New(fakeRunner{out: []byte("Volume: 75%\n")}, nil, b)
```

### Service structure

```go
type Service struct {
    runner runner.Runner
    bus    *bus.Bus
    last   state.AudioState  // change deduplication
}

func New(r runner.Runner, sr runner.StreamReader, b *bus.Bus) *Service {
    return &Service{runner: r, bus: b}
}

// NewWithDefaults is the convenience constructor used in production.
func NewWithDefaults(b *bus.Bus) *Service {
    return New(runner.New(), runner.NewStreamReader(), b)
}

func (s *Service) Run(ctx context.Context) error {
    // Option A: event-driven stream
    rc, err := s.stream.Stream("pactl", "subscribe")
    // ... read lines, publish on change

    // Option B: polling — use runner.PollLoop, never hand-roll a ticker
    return runner.PollLoop(ctx, 2*time.Second, s.poll)
}
```

**Change deduplication:** compare new state to `s.last` before publishing. Skip the publish if nothing changed. This keeps the bus quiet and prevents unnecessary GTK redraws.

### DBus services

DBus services receive a `*dbus.Conn` injected at construction (either system or session bus). The `internal/dbusutil` package provides a `DBusConn` interface so tests can inject a fake connection without a real bus running.

### Registering a new service

1. Create `internal/services/<name>/<name>.go`
2. Define `Service` with injected dependencies behind interfaces
3. Add `New(deps..., b *bus.Bus)` and `NewWithDefaults(b *bus.Bus)`
4. Implement `Run(ctx context.Context) error`
5. Publish to a bus topic on state change
6. Add the state type to `internal/state/state.go`
7. Add the topic constant to `internal/bus/bus.go`
8. Add a field to `internal/servicerefs/servicerefs.go`
9. Instantiate and start with `go refs.MyService.Run(ctx)` in `surfaces/app.go`

---

## Surface Patterns (`surfaces/`)

### Layer-shell windows

Every surface is a `gtk.ApplicationWindow` configured via `layershell.NewWindow`:

```go
win := layershell.NewWindow(app, layershell.WindowConfig{
    Name:          "snry-mysurface",                    // CSS widget name
    Layer:         layershell.LayerOverlay,             // see table below
    Anchors:       layershell.FullscreenAnchors(),      // or TopEdgeAnchors(), custom map
    KeyboardMode:  layershell.KeyboardModeOnDemand,     // None / Exclusive / OnDemand
    ExclusiveZone: 0,    // >0 reserves pixels at edge; -1 fills over everything
    Namespace:     "snry-mysurface",
})
```

Layer reference:
| Layer | Use |
|---|---|
| `LayerBackground` | Wallpaper-level, below all windows |
| `LayerBottom` | Below normal windows, above background |
| `LayerTop` | Above normal windows (bar, OSD) |
| `LayerOverlay` | Above everything, including other shell surfaces |

`ExclusiveZone > 0` on the bar tells the compositor to shrink the usable area so maximised windows don't overlap the bar. Use `-1` for overlays that should cover the whole screen.

### Popup panels

Most popups share the same structure. Use `surfaceutil.NewPopupPanel` instead of building it by hand:

```go
win, scrim, root := surfaceutil.NewPopupPanel(app, b, surfaceutil.PopupPanelConfig{
    Name:      "snry-mypopup",
    Namespace: "snry-mypopup",
    CloseOn:   []string{"toggle-wifi", "toggle-bluetooth", "toggle-calendar"},
    Align:     gtk.AlignEnd,  // AlignStart for left-aligned, AlignEnd for right
})
// Append your panel widget into root.
root.Append(myPanelWidget)
```

`NewPopupPanel` sets up:
- Fullscreen overlay with click-to-close scrim
- Escape-to-close keyboard handler
- Top margin that tracks bar height dynamically via `layershell.OnBarHeightChanged`
- Auto-close when any of the `CloseOn` sibling actions fire

To position the popup under a bar trigger widget:

```go
b.Subscribe(bus.TopicPopupTrigger, func(e bus.Event) {
    pt, ok := e.Data.(surfaceutil.PopupTrigger)
    if !ok || pt.Action != "toggle-mypopup" { return }
    p.trigger = pt.Trigger
    p.monitor = pt.Monitor
})

b.Subscribe(bus.TopicSystemControls, func(e bus.Event) {
    if e.Data != "toggle-mypopup" { return }
    glib.IdleAdd(func() {
        surfaceutil.PositionUnderTrigger(root, p.trigger, panelWidth, panelMargin, p.monitor)
        win.SetVisible(!win.Visible())
    })
})
```

Bar buttons publish a `surfaceutil.PopupTrigger` to `bus.TopicPopupTrigger` before publishing the toggle action to `bus.TopicSystemControls`. This gives the popup time to update its trigger reference before computing the position.

### GTK thread safety

GTK is single-threaded. **All widget mutations must run on the GTK main thread.** Service callbacks and bus handlers run on goroutines — always wrap:

```go
b.Subscribe(bus.TopicAudio, func(e bus.Event) {
    st := e.Data.(state.AudioState)
    glib.IdleAdd(func() {
        // Safe to touch GTK widgets here
        label.SetText(fmt.Sprintf("%.0f%%", st.Volume*100))
    })
})
```

Never call GTK functions directly from a `go func()` or a service `Run` goroutine without `glib.IdleAdd`.

### Toggle / visibility helpers

```go
// Toggle visibility when action fires on bus.TopicSystemControls
surfaceutil.AddToggleOn(b, win, "toggle-mysurface")

// Same, but also grabs keyboard focus when shown
surfaceutil.AddToggleOnWithFocus(b, win, "toggle-mysurface")

// Close on Escape key
surfaceutil.AddEscapeToClose(win)

// Close on Escape with a custom callback (e.g. to clean up state)
surfaceutil.AddEscapeToCloseWithCallback(win, func() { resetSearch() })
```

### Registering a new surface

1. Create `surfaces/<name>/<name>.go`
2. Implement `New(app *gtk.Application, b *bus.Bus, refs *servicerefs.ServiceRefs) *MySurface`
3. Configure the layer-shell window with `layershell.NewWindow`
4. Build the widget hierarchy in a private `build()` method
5. Subscribe to bus topics for data updates
6. Add the toggle command to `internal/controlsocket/controlsocket.go` so it can be triggered via `--toggle-*` flags
7. Instantiate it inside `app.ConnectActivate` in `surfaces/app.go`
8. Add CSS classes to `assets/style.css`

---

## Persistent Store (`internal/store`)

The store is a flat JSON key-value file at `~/.config/snry-shell/store.json`. Use it for values that must survive restarts but don't belong in the typed `settings.Config`.

```go
// Read with a default fallback (generic helper)
val := store.LookupOr("my.key", "default-value")

// Write
if err := store.Set("my.key", someValue); err != nil {
    log.Printf("store: %v", err)
}

// Read into an existing typed variable
var v MyType
ok := store.Get("my.key", &v)

// Batch write — single file flush for multiple keys
_ = store.SetMany(map[string]any{
    "key.a": 1,
    "key.b": "hello",
})
```

Use dot-separated namespaces for keys to avoid collisions (e.g. `"theme.wallpaper"`, `"pomodoro.duration"`). For settings that belong in `settings.Config` (user-facing shell behaviour), add a constant key and read/write via `settings.Load` / `settings.Save` instead.

---

## Layershell CGo Bindings (`internal/layershell`)

The bindings use a hand-written thin CGo layer rather than a generated wrapper. This avoids compatibility issues between the gotk4 object model and older gtk-layer-shell Go bindings.

**How it works:**

1. `#cgo pkg-config: gtk4-layer-shell-0` in the CGo preamble resolves include paths and linker flags automatically.
2. Each C function is forward-declared in the preamble — no header `#include` required.
3. GTK object pointers are extracted via `Native()` (returns `uintptr`) and cast to the C struct pointer with `unsafe.Pointer`.
4. `WindowPtr(w any)` extracts `*C.GtkWindow` from any gotk4 type that implements `Native() uintptr`.

When adding new gtk4-layer-shell calls, forward-declare the C function in the preamble and add a Go wrapper that calls `WindowPtr` and passes the result to C.

**Touch cursor tracker** (`installTouchCursorTracker`) is attached to every `NewWindow` call. It hides the mouse cursor when the active input device is a touchscreen and restores it on mouse motion, making the shell usable as a tablet UI without cursor artifacts.

**Bar height propagation:** `layershell/defaults.go` holds a process-global `barHeight` variable updated by the bar on every size allocation. Any surface needing the current bar height calls `layershell.BarHeight()` or registers with `layershell.OnBarHeightChanged`. This avoids threading the bar window reference into every surface constructor.

---

## Theme System (`internal/theme`)

The theme system has two components:

**`Generator`** reads a wallpaper image, samples a grid of pixels across the image, quantizes the samples to ~8 dominant colors using HSL distance bucketing, then derives a full Material Design 3 color scheme (30+ tokens) covering primary, secondary, tertiary, surface, background, error, and outline roles in both dark and light variants. The output is a CSS file written to `~/.cache/snry-shell/theme.css` containing `@define-color` overrides for the base stylesheet variables.

**`Monitor`** runs as a background goroutine and polls for wallpaper changes every 5 seconds. It detects the current wallpaper by querying hyprpaper, swww, nitrogen, and feh in that order. When the path changes it calls `Generator.SetWallpaper`, persists the path to the store under `"theme.wallpaper"`, and publishes `bus.TopicThemeChanged`. `surfaces/app.go` subscribes and reloads the GTK CSS provider on the main thread via `glib.IdleAdd`.

To force a regeneration without restarting:
```sh
snry-shell --toggle-reload-theme
```

---

## Styling (`assets/style.css`)

The stylesheet is embedded into the binary at compile time via `assets/embed.go` (`//go:embed style.css`) and loaded as a GTK CSS provider at startup. Theme-generated colors override it at a higher CSS priority.

**Adding styles for a new surface:**

1. Add CSS classes to your widget hierarchy: `box.AddCSSClass("my-surface")`.
2. Add rules to `assets/style.css`. Use the `@define-color` tokens for all colors — never hardcode hex values.
3. Inspect the live widget tree with `GTK_DEBUG=interactive ./snry-shell`.

**User overrides:** if `~/.config/snry-shell/custom.css` exists it is loaded last at the highest priority, giving users a clean override point.

---

## Adding Quick Toggles (`surfaces/widgets/toggles.go`)

Quick toggles are entries in the `toggles []toggleDef` slice. Each entry specifies:

| Field | Purpose |
|---|---|
| `icon` | Material Symbols icon name rendered as text |
| `label` | Display label |
| `topic` | Bus topic for state sync (optional — only for state-tracking toggles) |
| `requires` | Binary name checked with `exec.LookPath`; toggle hidden if absent |
| `button` | Rendered as a plain button with no active state if true |
| `segmented` | Replaced with the segmented input-mode control if true |
| `toggle` | Called with `(active bool)` on click |
| `longPress` | Called on long press (used by WiFi/Bluetooth to open their picker popup) |

State-tracking toggles (WiFi, Bluetooth, Night Light, DND) subscribe to the relevant bus topic and call `tb.SetActive(...)` to reflect current system state without triggering the `toggle` callback (guarded by a `settingState` flag).

---

## Pull Requests

1. Fork the repository and create a feature branch off `main`.
2. Keep commits focused — one logical change per commit.
3. Run `make vet && make test` before opening a PR.
4. Follow the existing commit message style: `<package>: <imperative summary>` (e.g. `audio: debounce rapid volume events`).
5. Surface UI packages cannot be unit-tested without a display — that is expected. Test the logic in `internal/` packages instead.
