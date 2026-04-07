# snry-shell

[![CI](https://github.com/sonroyaalmerol/snry-shell/actions/workflows/ci.yml/badge.svg)](https://github.com/sonroyaalmerol/snry-shell/actions/workflows/ci.yml)
[![Release](https://github.com/sonroyaalmerol/snry-shell/actions/workflows/release.yml/badge.svg)](https://github.com/sonroyaalmerol/snry-shell/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/sonroyaalmerol/snry-shell.svg)](https://pkg.go.dev/github.com/sonroyaalmerol/snry-shell)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A Wayland desktop shell panel built with Go, GTK4, and gtk4-layer-shell for the Hyprland compositor.

## Overview

snry-shell provides a complete desktop shell UI layer:

- **Status bar** — workspaces, window title, notification unread badge, system tray (SNI), resource monitor, keyboard layout, volume/brightness/network/battery indicators, clock
- **Application overview** — window previews grouped by workspace, fuzzy app launcher
- **Notifications** — freedesktop notification daemon (DBus), toast popups, sidebar notification list
- **Control panel** — media controls, calendar popup, quick toggles (13 total), pomodoro timer, todo list, volume mixer, WiFi picker, Bluetooth picker
- **Quick toggles** — WiFi, Bluetooth, Night Light, Anti-Flash, Mic Mute, EasyEffects, DND, Idle Off, GameMode, Performance, Screenshot, Color Pick, On-Screen Keyboard
- **Lock screen** — password entry, clock display
- **Session menu** — lock, suspend, reboot, shutdown, logout
- **Settings panel** — dark mode toggle (overlay, `--toggle-settings`)
- **Standalone control panel** — full-window settings UI launched with `--control-panel` or `-c`
- **Wallpaper picker** — grid browser with automatic Material Design 3 theming (built-in, no external tools required)
- **Clipboard history** — searchable panel with cliphist integration
- **Emoji picker** — categorized emoji grid with wl-copy
- **Notes overlay** — persistent text notes (auto-saved to disk)
- **Screen recorder** — wf-recorder integration with live timer
- **Floating image viewer** — click-to-dismiss image display
- **Polkit agent** — GUI authentication dialog (replaces text-based agent)
- **On-screen keyboard** — QWERTY layout with key injection via `/dev/uinput`, auto-show via `zwp_input_method_v2`
- **Window management popup** — grouped window list with per-workspace navigation
- **Extras** — screen corner hotspots, crosshair overlay, region screenshot selector, cheatsheet, OSD (volume/brightness)

## Architecture

```
cmd/snry-shell/main.go         CLI entry point (control socket / app launch / --control-panel)
surfaces/
  app.go                       Application orchestrator
  bar/                          Status bar surface
  overview/                     App launcher + window grid
  popup/
    appdrawer/                  App drawer popup
    calendar/                   Calendar popup
    notifcenter/               Notification center sidebar
    wifi/                       WiFi picker popup
    bluetooth/                  Bluetooth picker popup
    windowmgmt/                 Window management popup (grouped by workspace)
  controlpanel/                 Standalone settings UI (--control-panel / -c flag)
  lockscreen/                   Lock screen
  session/                      Power menu
  settings/                     Settings overlay (dark mode toggle)
  mediaoverlay/                 Full-screen media controls
  clipboard/                    Clipboard history panel
  emoji/                        Emoji picker overlay
  notes/                        Notes overlay
  recorder/                     Screen recorder controls
  imageviewer/                  Floating image viewer
  polkit/                       PolicyKit authentication agent
  osd/                          Volume/brightness on-screen display
  notifpopup/                   Notification toast popups
  osk/                          On-screen keyboard
  regionselector/               Region screenshot tool
  corners/                      Screen corner hotspots
  crosshair/                    Crosshair overlay
  cheatsheet/                   Keyboard shortcuts overlay
  widgets/                      Shared bar/panel widgets (toggles, sliders, media, etc.)
internal/
  bus/                          Event bus (pub/sub with replay)
  state/                        Shared state types
  store/                        Persistent JSON key-value store (~/.config/snry-shell/store.json)
  servicerefs/                  Service container struct
  services/
    hyprland/                   Hyprland IPC (socket events) + queries (hyprctl)
    audio/                      Volume control (wpctl; event-driven via pactl subscribe)
    brightness/                 Brightness control (pure Go DDC/CI over I2C)
    resources/                  CPU/RAM monitoring (/proc, change detection skips <1% deltas)
    audiomixer/                 Per-app volume (pactl; event-driven via pactl subscribe)
    network/                    WiFi scanning + connectivity (NetworkManager DBus)
    bluetooth/                  Device discovery + pairing (Bluez DBus)
    mpris/                      Media player control (MPRIS2 DBus, PropertiesChanged + Seeked signals)
    upower/                     Battery status (UPower DBus)
    notifications/              Notification server (freedesktop DBus)
    clipboard/                  Clipboard history (cliphist; event-driven via wl-paste --watch)
    nightmode/                  Night light (hyprsunset)
    darkmode/                   Dark mode detection (xdg-desktop-portal / gsettings fallback)
    inputmode/                  Input mode switching (auto/tablet/desktop)
    pomodoro/                   Pomodoro timer (internal)
    todo/                       Task list (JSON persistence)
    sni/                        System tray host (StatusNotifierItem DBus)
    runner/                     Command abstraction (Runner, StreamReader, Commander, PollLoop)
  ddc/                          Pure Go DDC/CI monitor control (I2C ioctls, bus caching)
  inputmethod/                  zwp_input_method_v2 watcher for OSK auto-show
  controlsocket/                Unix socket for --toggle-* commands
  dbusutil/                     D-Bus helper utilities
  fileutil/                     File I/O helpers
  gtkutil/                      Shared GTK widget helpers
  surfaceutil/                  Shared layer-shell surface helpers
  uinput/                       Virtual keyboard via /dev/uinput (zero-dependency key injection)
  settings/                     User configuration (backed by store)
  theme/                        Built-in M3 color scheme extraction + wallpaper monitor
  calendar/                     Calendar grid logic
  launcher/                     XDG .desktop loader + fuzzy search
  layershell/                   CGo bindings for gtk4-layer-shell
assets/
  embed.go                      Embeds style.css into the binary
  style.css                     Material Design 3 base stylesheet
```

### Design Patterns

- **Event bus** — All services publish state changes to a central `bus.Bus`; surfaces subscribe to topics they care about. Late subscribers receive the last published event (replay). No direct service-to-surface coupling.
- **Event-driven architecture** — Services use event streams (pactl subscribe, wl-paste --watch, MPRIS D-Bus signals, Hyprland socket events, zwp_input_method_v2) instead of polling where possible. Remaining polling services (brightness via I2C, resources via /proc, darkmode, theme) use change deduplication to skip redundant publishes.
- **Dependency injection** — Every service that touches the OS (subprocesses, sockets, DBus, I2C ioctls) is abstracted behind an interface, enabling unit tests with fakes.
- **Layer shell** — Each surface is a separate gtk-layer-shell window with its own layer, anchors, exclusive zone, and keyboard mode.
- **Service refs** — A single `ServiceRefs` struct bundles all service pointers and is passed to surface constructors that need them.
- **Persistent store** — Settings and state are persisted in a typed JSON key-value store (`~/.config/snry-shell/store.json`) rather than separate config files.
- **Forced configs** — The shell temporarily injects Hyprland config values (e.g. `decoration:rounding`) via `hyprctl keyword` and restores originals on exit.

## Installation

### From package

Download the latest release from [GitHub Releases](https://github.com/sonroyaalmerol/snry-shell/releases). Packages are available for:

| Format | Distro |
|--------|--------|
| `.deb` | Debian, Ubuntu, Pop!_OS |
| `.rpm` | Fedora, RHEL, openSUSE |
| `.apk` | Alpine Linux |
| AUR | Arch Linux |

#### Arch Linux (AUR)

```
yay -S snry-shell-bin
```

#### Debian / Ubuntu

```
sudo dpkg -i snry-shell_<version>_linux_amd64.deb
```

#### Fedora

```
sudo rpm -i snry-shell_<version>_linux_x86_64.rpm
```

### From source

See [Building](#building) below.

## Prerequisites

- **Go** 1.26+
- **Hyprland** compositor
- **gtk4-layer-shell** development headers
- **System tools**: pkgconf, wpctl (wireplumber), grim, wl-copy, cliphist, hyprpicker, wf-recorder
- **I2C access**: User must be in the `i2c` group for direct monitor brightness control
- **Optional**: swww / hyprpaper / nitrogen / feh (for wallpaper detection), checkpw (lock screen PAM), polkit-agent-helper-1 (polkit authentication)
- **Fonts**: Google Sans Flex, Material Symbols Rounded, JetBrains Mono NF

### Arch Linux

```sh
# Build dependencies
sudo pacman -S --needed gtk4 gtk4-layer-shell pkgconf go

# Runtime dependencies (official repos)
sudo pacman -S --needed wireplumber grim wl-clipboard wf-recorder polkit

# Runtime dependencies (AUR)
yay -S swww cliphist hyprpicker

# Fonts (AUR)
yay -S ttf-google-sans ttf-material-symbols-variable-git
```

### Fedora / RHEL

```sh
# Enable COPR for gtk4-layer-shell
sudo dnf copr enable solopash/hyprland

# Build dependencies
sudo dnf install gtk4-devel gtk4-layer-shell-devel pkgconf go gcc

# Runtime dependencies (official repos)
sudo dnf install wireplumber grim wl-clipboard wf-recorder

# Runtime dependencies (COPR or build from source)
# swww hyprpicker cliphist

# polkit
sudo dnf install polkit
```

### Debian / Ubuntu

```sh
# Build dependencies
sudo apt install libgtk-4-dev libgtk4-layer-shell-dev pkg-config golang gcc

# Runtime dependencies (official repos)
sudo apt install wireplumber grim wl-clipboard cliphist \
    hyprpicker wf-recorder checkpw polkitd

# Runtime dependencies (build from source)
cargo install swww

# Fonts (install manually from Google Fonts / Nerd Fonts)
```

### openSUSE (Tumbleweed)

```sh
# Build dependencies
sudo zypper install gtk4-devel gtk4-layer-shell-devel pkg-config go gcc

# Runtime dependencies (official repos)
sudo zypper install swww wireplumber grim wl-clipboard cliphist \
    hyprpicker wf-recorder polkit

# Fonts (install manually from Google Fonts / Nerd Fonts)
```

### Alpine Linux

```sh
# Build dependencies
sudo apk add gtk4.0-dev gtk4-layer-shell-dev pkgconf go gcc musl-dev

# Runtime dependencies (official repos)
sudo apk add swww wireplumber grim wl-clipboard wf-recorder polkit

# Runtime dependencies (build from source)
cargo install cliphist hyprpicker

# Fonts (install manually from Google Fonts / Nerd Fonts)
```

## Building

```
make build
```

Or directly:

```
go build -o snry-shell ./cmd/snry-shell/
```

To install to `$GOPATH/bin`:

```
make install
```

## Running

Add to your Hyprland config:

```
exec-once = snry-shell
```

### Keybind toggles

The binary accepts `--toggle-*` flags that send commands to the running instance via a Unix socket:

```
bind = SUPER, space,  exec, snry-shell --toggle-overview
bind = SUPER, escape, exec, snry-shell --toggle-controls
bind = SUPER, Q,      exec, snry-shell --toggle-session
bind = SUPER, P,      exec, snry-shell --toggle-settings
bind = SUPER, S,      exec, snry-shell --toggle-region-selector
bind = SUPER, K,      exec, snry-shell --toggle-osk
bind = SUPER, M,      exec, snry-shell --toggle-media-overlay
bind = SUPER, V,      exec, snry-shell --toggle-clipboard
bind = SUPER, E,      exec, snry-shell --toggle-emoji
bind = SUPER, N,      exec, snry-shell --toggle-notes
```

### Standalone control panel

Launch a full-window settings UI without starting the shell:

```
snry-shell --control-panel
# or
snry-shell -c
```

### Control socket

snry-shell listens on `/tmp/snry-shell.sock`. Any `toggle-*` command sent to the socket is dispatched via the event bus. Surfaces that handle a command toggle their visibility:

| Command | Surface |
|---------|---------|
| `toggle-overview` | Application overview |
| `toggle-controls` | Control panel |
| `toggle-notif-center` | Notification center |
| `toggle-calendar` | Calendar popup |
| `toggle-session` | Power menu |
| `toggle-crosshair` | Crosshair overlay |
| `toggle-cheatsheet` | Keyboard shortcuts |
| `toggle-media-overlay` | Full-screen media controls |
| `toggle-settings` | Settings panel |
| `toggle-region-selector` | Region screenshot selector |
| `toggle-osk` | On-screen keyboard |
| `toggle-clipboard` | Clipboard history |
| `toggle-emoji` | Emoji picker |
| `toggle-notes` | Notes overlay |
| `toggle-recorder` | Screen recorder |
| `toggle-reload-theme` | Force theme regeneration |

## Configuration

Settings are persisted in the key-value store at `~/.config/snry-shell/store.json`. The following keys are used:

| Key | Default | Description |
|-----|---------|-------------|
| `dark_mode` | `true` | Enable dark color scheme |
| `do_not_disturb` | `false` | Suppress notification toasts |
| `input_mode` | `"auto"` | Input mode: `"auto"`, `"tablet"`, or `"desktop"` |
| `theme.wallpaper` | `""` | Last detected wallpaper path (auto-updated) |

Settings can be changed from the built-in settings panel (`--toggle-settings`) or the standalone control panel (`--control-panel`).

## Theming

snry-shell uses **Material Design 3** color tokens with built-in dynamic color extraction from your wallpaper. No external tools are required.

### How it works

The shell automatically detects your wallpaper from common tools (hyprpaper, swww, nitrogen, feh), extracts dominant colors via grid sampling and HSL-space manipulation, and generates a Material Design 3 color scheme. Dynamic color variables are written to `~/.cache/snry-shell/theme.css` and hot-reloaded whenever the wallpaper changes (polled every 5 seconds).

### Manual theme refresh

Force theme regeneration from current wallpaper:
```
snry-shell --toggle-reload-theme
```

### Color tokens

The base stylesheet uses `@define-color` variables that the theme generator overrides:

- `@col_primary`, `@col_on_primary`, `@col_primary_container`, ...
- `@col_surface`, `@col_on_surface`, `@col_surface_container`, ...
- `@col_background`, `@col_on_background`, `@col_outline`, ...

Custom CSS overrides can be added to `~/.config/snry-shell/custom.css` (create if needed).

## Testing

```
make test
```

Test suites cover the internal packages (bus, calendar, ddc, launcher, services, settings, theme, uinput, controlsocket, osk). DDC tests include an optional hardware integration test (`DDC_INTEGRATION=1`). Surface UI packages require a GTK display and are not unit-tested.

## License

[MIT](LICENSE)
