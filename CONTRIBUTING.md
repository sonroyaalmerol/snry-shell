# Contributing to snry-shell

## Development Setup

### System dependencies

**Arch Linux:**
```
pacman -S go gtk4 gtk4-layer-shell pkg-config swww matugen wireplumber grim wl-clipboard wtype mako
```

**Fedora:**
```
dnf install go gtk4-devel gtk4-layer-shell-devel pkg-config
```

### Fonts

snry-shell uses Material Symbols for icons. Install these fonts:
- [Google Sans Flex](https://fonts.google.com/specimen/Google+Sans+Flex)
- [Material Symbols Rounded](https://fonts.google.com/specimen/Material+Symbols+Rounded)
- [JetBrains Mono NF](https://www.nerdfonts.com/font-downloads)

### Build

```
make build
./snry-shell
```

### Test

```
make test
```

### Lint

```
make vet
make fmt
```

## Project Structure

```
cmd/snry-shell/    Entry point (CLI)
surfaces/           GTK4 UI surfaces (each surface is a sub-package)
internal/
  services/         Backend services (each service is a sub-package)
  bus/              Event bus
  state/            Shared state types
  servicerefs/      Service container
  settings/         Configuration
  theme/            Color scheme utilities
  calendar/         Calendar logic
  launcher/         App search + fuzzy matching
  layershell/       CGo layer-shell bindings
assets/style.css    Base stylesheet
```

## Code Conventions

### Packages

- Each surface is its own package under `surfaces/<name>/`
- Each service is its own package under `internal/services/<name>/`
- All inter-component communication goes through the event bus (`internal/bus/`)
- State types live in `internal/state/`
- The service container is `internal/servicerefs/servicerefs.go`

### Services

Services that touch the OS (subprocesses, sockets, DBus) must abstract their dependencies behind interfaces for testability:

```go
// Runner starts a subprocess.
type Runner interface {
    Output(args ...string) ([]byte, error)
}
```

Each service follows this pattern:
- `New(dependency, bus) *Service` — constructor
- `Run(ctx) error` — main loop (polls/reads/watches)
- Methods for actions (e.g., `SetVolume`, `Toggle`)
- Publishes state changes to the bus

### Surfaces

Each surface follows this pattern:
- `New(app, bus, refs?) *Surface` — constructor that creates the layer-shell window
- Private `build()` method for widget hierarchy
- Subscribes to bus topics for data updates
- Uses `glib.IdleAdd()` for all GTK calls from goroutines

### Style

- Run `gofmt` and `go vet` before submitting
- Use `goimports` for import ordering
- Add doc comments to all exported types and functions
- Add a package doc comment to every package

## Adding a New Service

1. Create `internal/services/<name>/<name>.go`
2. Define a `Service` struct with a `Runner` or `DBusConn` interface for testability
3. Implement `New(deps, bus) *Service` and `Run(ctx) error`
4. Publish state to a bus topic
5. Add state type to `internal/state/state.go`
6. Add topic constant to `internal/bus/bus.go`
7. Add field to `internal/servicerefs/servicerefs.go`
8. Register the service in `surfaces/app.go` → `startServices()`

## Adding a New Surface

1. Create `surfaces/<name>/<name>.go`
2. Implement `New(app *gtk.Application, b *bus.Bus, refs?) *Surface`
3. Configure the layer-shell window (layer, anchors, keyboard mode, exclusive zone)
4. Build the widget hierarchy in a private `build()` method
5. Subscribe to bus topics as needed
6. Add the toggle command to `surfaces/app.go` → `handleControl()`
7. Add CSS classes to `assets/style.css`

## Adding a Sidebar Widget

1. Create `surfaces/sidebar/<name>.go`
2. Implement a public constructor function (e.g., `newClockWidget(b *bus.Bus) gtk.Widgetter`)
3. Subscribe to the relevant bus topic
4. Wire into `surfaces/sidebar/right.go` → `build()`

## Pull Requests

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Ensure `make vet` and `make test` pass
5. Open a pull request
