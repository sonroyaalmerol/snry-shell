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
- **Control panel** — media controls, calendar popup, quick toggles (14 total), pomodoro timer, todo list, volume mixer, WiFi picker, Bluetooth picker
- **Quick toggles** — WiFi, Bluetooth, Night Light, Anti-Flashbang, Mic Mute, EasyEffects, Volume Mixer, DND, Idle Inhibitor, GameMode, Performance, Screenshot, Color Picker, WiFi Networks
- **Lock screen** — password entry, clock display
- **Session menu** — lock, suspend, reboot, shutdown, logout
- **Settings panel** — dark mode, font scale, bar position
- **Wallpaper picker** — grid browser with automatic Material Design 3 theming via matugen
- **Clipboard history** — searchable panel with cliphist integration
- **Emoji picker** — categorized emoji grid with wl-copy
- **Notes overlay** — persistent text notes (auto-saved to disk)
- **Screen recorder** — wf-recorder integration with live timer
- **Floating image viewer** — click-to-dismiss image display
- **Polkit agent** — GUI authentication dialog (replaces text-based agent)
- **On-screen keyboard** — QWERTY layout with key injection via `/dev/uinput` (zero external dependencies)
- **Extras** — screen corner hotspots, crosshair overlay, region screenshot selector, cheatsheet, OSD (volume/brightness)

## Architecture

```
cmd/snry-shell/main.go         CLI entry point (control socket / app launch)
surfaces/
  app.go                       Application orchestrator
  bar/                          Status bar surface
  overview/                     App launcher + window grid
  popup/
    controls/                   Control panel (toggles, sliders, media, widgets)
    calendar/                   Calendar popup
    notifcenter/               Notification center sidebar
  lockscreen/                   Lock screen
  session/                      Power menu
  settings/                     Settings panel
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
  bus/                          Event bus (pub/sub)
  state/                        Shared state types
  servicerefs/                  Service container struct
  services/
    hyprland/                   Hyprland IPC + queries + forced config injection
    audio/                      Volume control (wpctl)
    brightness/                 Brightness control (brightnessctl)
    resources/                  CPU/RAM monitoring (/proc)
    audiomixer/                 Per-app volume (pactl)
    network/                    WiFi scanning + connectivity (NetworkManager DBus)
    bluetooth/                  Device discovery + pairing (Bluez DBus)
    mpris/                      Media player control (MPRIS2 DBus)
    upower/                     Battery status (UPower DBus)
    notifications/              Notification server (freedesktop DBus)
    clipboard/                  Clipboard history (wl-clipboard)
    wallpaper/                  Wallpaper watcher (swww)
    nightmode/                  Night light (hyprsunset)
    weather/                    Weather data
    pomodoro/                   Pomodoro timer
    todo/                       Task list (JSON persistence)
    sni/                        System tray host (StatusNotifierItem DBus)
    runner/                     Command abstraction for testable subprocess calls
  atspi2/                       AT-SPI2 text input focus detection (accessibility D-Bus)
  controlsocket/                Unix socket for --toggle-* commands
  dbusutil/                     D-Bus helper utilities
  fileutil/                     File I/O helpers
  gtkutil/                      Shared GTK widget helpers
  surfaceutil/                  Shared layer-shell surface helpers
  uinput/                       Virtual keyboard via /dev/uinput (zero-dependency key injection)
  settings/                     User configuration (JSON)
  theme/                        M3 color scheme utilities
  calendar/                     Calendar grid logic
  launcher/                     XDG .desktop loader + fuzzy search
  layershell/                   CGo bindings for gtk4-layer-shell
assets/
  embed.go                      Embeds style.css into the binary
  style.css                     Material Design 3 base stylesheet
```

### Design Patterns

- **Event bus** — All services publish state changes to a central `bus.Bus`; surfaces subscribe to topics they care about. No direct service-to-surface coupling.
- **Dependency injection** — Every service that touches the OS (subprocesses, sockets, DBus) is abstracted behind an interface, enabling unit tests with fakes.
- **Layer shell** — Each surface is a separate gtk-layer-shell window with its own layer, anchors, exclusive zone, and keyboard mode.
- **Service refs** — A single `ServiceRefs` struct bundles all service pointers and is passed to surface constructors that need them.
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
- **System tools**: pkgconf, swww, matugen, wpctl (wireplumber), grim, wl-copy, cliphist, hyprpicker, wf-recorder
- **Optional**: checkpw (lock screen PAM), polkit-agent-helper-1 (polkit authentication)
- **Fonts**: Google Sans Flex, Material Symbols Rounded, JetBrains Mono NF

### Arch Linux

```sh
# Build dependencies
sudo pacman -S --needed gtk4 gtk4-layer-shell pkgconf go

# Runtime dependencies (official repos)
sudo pacman -S --needed wireplumber grim wl-clipboard wf-recorder polkit

# Runtime dependencies (AUR)
yay -S swww matugen-bin cliphist hyprpicker

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
yay -S swww matugen hyprpicker cliphist

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
cargo install swww matugen

# Fonts (install manually from Google Fonts / Nerd Fonts)
```

### openSUSE (Tumbleweed)

```sh
# Build dependencies
sudo zypper install gtk4-devel gtk4-layer-shell-devel pkg-config go gcc

# Runtime dependencies (official repos)
sudo zypper install swww wireplumber grim wl-clipboard cliphist \
    hyprpicker wf-recorder polkit

# Runtime dependencies (build from source)
cargo install matugen

# Fonts (install manually from Google Fonts / Nerd Fonts)
```

### Alpine Linux

```sh
# Build dependencies
sudo apk add gtk4.0-dev gtk4-layer-shell-dev pkgconf go gcc musl-dev

# Runtime dependencies (official repos)
sudo apk add swww wireplumber grim wl-clipboard wf-recorder polkit

# Runtime dependencies (build from source)
cargo install matugen cliphist hyprpicker

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

### Control socket

snry-shell listens on `/tmp/snry-shell.sock`. The following commands are accepted:

| Command | Action |
|---------|--------|
| `toggle-overview` | Toggle application overview |
| `toggle-controls` | Toggle control panel |
| `toggle-session` | Toggle session power menu |
| `toggle-crosshair` | Toggle crosshair overlay |
| `toggle-cheatsheet` | Toggle keyboard shortcuts overlay |
| `toggle-media-overlay` | Toggle full-screen media controls |
| `toggle-settings` | Toggle settings panel |
| `toggle-region-selector` | Toggle region screenshot selector |
| `toggle-osk` | Toggle on-screen keyboard |
| `toggle-clipboard` | Toggle clipboard history panel |
| `toggle-emoji` | Toggle emoji picker |
| `toggle-notes` | Toggle notes overlay |
| `toggle-recorder` | Toggle screen recorder |

## Configuration

Settings are stored at `~/.config/snry-shell/settings.json`:

```json
{
  "dark_mode": true,
  "font_scale": 1.0,
  "bar_position": "top",
  "do_not_disturb": false,
  "wallpaper_dir": "~/Pictures/Wallpapers"
}
```

Settings can be changed from the built-in settings panel (toggle via `--toggle-settings`).

## Theming

snry-shell uses **Material Design 3** color tokens. The theme is automatically generated from your wallpaper via [matugen](https://github.com/matugen/matugen). Dynamic color variables are written to `~/.cache/snry-shell/theme.css` and hot-reloaded whenever the wallpaper changes.

Custom CSS overrides can be added to `assets/style.css`. The base stylesheet uses `@define-color` variables that matugen overrides:

- `@col_primary`, `@col_on_primary`, `@col_primary_container`, ...
- `@col_surface`, `@col_on_surface`, `@col_surface_container`, ...
- `@col_background`, `@col_on_background`, `@col_outline`, ...

## Testing

```
make test
```

Test suites cover the internal packages (bus, calendar, launcher, services, settings, theme, uinput, atspi2, osk). Surface UI packages require a GTK display and are not unit-tested.

## License

[MIT](LICENSE)
