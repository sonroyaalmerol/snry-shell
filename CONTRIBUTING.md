# Contributing to snry-shell

## Development Setup

### System dependencies

**Arch Linux:**
```sh
sudo pacman -S go gtk4 gtk4-layer-shell pkgconf
yay -S swww cliphist hyprpicker
sudo pacman -S wireplumber grim wl-clipboard wf-recorder polkit
```

### Protocol Bindings

This project uses standard Wayland protocols. If you modify or add XML protocol files in `internal/services/*/protocol/`, regenerate the Go bindings:

```sh
go run github.com/rajveermalviya/go-wayland/cmd/go-wayland-scanner@latest \
    -i /path/to/protocol.xml \
    -o internal/services/myservice/protocol/protocol.go \
    -pkg protocol
```

**Note:** Always use `waylandutil.FixedBind` instead of `registry.Bind` when initializing new global interfaces to avoid string padding bugs in the underlying library.

## Architecture & Data Flow

```
cmd/snry-shell/main.go     CLI: flags dispatch to surfaces.Run() or controlpanel.Run()
surfaces/app.go           The "Main" loop: initialises all services and surfaces
surfaces/*/               One package per layer-shell window (Bar, Overview, etc.)
internal/bus/              Pub/sub event bus — the only coupling between services and surfaces
internal/state/            Plain data types for every published event payload
internal/services/*/       One package per backend service (audio, network, bluetooth, etc.)
internal/store/            Persistent key-value store (~/.config/snry-shell/store.json)
internal/settings/         Typed shell config (thin wrapper over store)
internal/layershell/       CGo bindings for gtk4-layer-shell
internal/surfaceutil/      Helpers shared by all surfaces (popup panels, toggles, geometry)
internal/waylandutil/      Workarounds for library bugs and Wayland helper functions
internal/gtkutil/          GTK widget helpers (M3 dialogs, buttons, sliders)
internal/theme/            Built-in wallpaper color extraction and CSS generation
internal/uinput/           Virtual keyboard via /dev/uinput (zero-dependency key injection)
internal/ddc/              Pure-Go DDC/CI brightness control (I2C ioctls)
assets/style.css           Base Material Design 3 stylesheet (embedded into the binary)
```

### 1. The Event Bus
We avoid direct dependencies. If the **Bar** needs to show the volume, it doesn't call `audioService.GetVolume()`. Instead:
1. `audio.Service` publishes `state.AudioState` to `bus.TopicAudio`.
2. `bar.VolumeWidget` subscribes to `bus.TopicAudio` and updates its label.

### 2. Persistent Store
Don't use `os.WriteFile` for settings. Use the typed store:

```go
// Read
val := store.LookupOr("my.key", defaultValue)

// Write
_ = store.SetMany(map[string]any{
    "key.a": 1,
    "key.b": "hello",
})
```

Use dot-separated namespaces for keys to avoid collisions (e.g. `"theme.wallpaper"`, `"shell.idle_timeout"`). For settings that belong in `settings.Config` (user-facing shell behaviour), add a constant key and read/write via `settings.Load` / `settings.Save` instead.

---

## Layershell CGo Bindings (`internal/layershell`)

The shell uses `gtk4-layer-shell`. We use a small internal wrapper to bridge the C library to Go. If you add new layer-shell features, update `layershell.go`.

## Code Style

1. **Surgical Changes:** Keep PRs focused. Avoid large refactors unless requested.
2. **Concurrency:** Always use `s.mu.Lock()` when accessing shared service state.
3. **GTK Threading:** All UI updates must happen on the main thread via `glib.IdleAdd`.
4. **Git Commits:** Follow the existing commit message style: `<package>: <imperative summary>` (e.g. `audio: debounce rapid volume events`).
5. **Testing:** Test logic in `internal/` packages. Surface UI packages cannot be unit-tested without a display.
