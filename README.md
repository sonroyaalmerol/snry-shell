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
- **Calendar & Quick Settings** — calendar popup combined with quick toggles (13 total), volume & brightness sliders
- **Quick toggles** — WiFi, Bluetooth, Night Light, Anti-Flash, Mic Mute, EasyEffects, DND, Idle Off, GameMode, Performance, Screenshot, Color Pick, Input Mode
- **Lock screen** — password entry, clock display, PAM integration. Fully supports **On-Screen Keyboard** for touch devices.
- **Session menu** — lock, suspend, reboot, shutdown, logout
- **Settings panel** — quick settings overlay (launched with `--toggle-settings`)
- **Standalone control panel** — full-window settings UI launched with `--control-panel` or `-c`
- **Dynamic Material Design 3 theming** — automatic color scheme extraction from wallpaper (no external tools required)
- **Notes overlay** — persistent text notes (auto-saved to disk)
- **Screen recorder** — wf-recorder integration with live timer
- **Floating image viewer** — click-to-dismiss image display
- **Polkit agent** — GUI authentication dialog (replaces text-based agent)
- **On-screen keyboard** — QWERTY layout with key injection via `/dev/uinput`, auto-show via `zwp_input_method_v2`. Includes built-in **Emoji picker** and **Clipboard history**.
- **Window management popup** — grouped window list with per-workspace navigation
- **Idle Service** — High-performance inactivity monitoring using the native `ext-idle-notify-v1` protocol. Supports automatic screen locking, DPMS display-off, and system suspension.

## Architecture

```
cmd/snry-shell/main.go         CLI entry point (control socket / app launch / --control-panel)
surfaces/
  app.go                       Application orchestrator
  bar/                          Status bar surface
  overview/                     App launcher + window grid
  popup/
    appdrawer/                  App drawer popup
    calendar/                   Calendar + Quick Settings popup
    notifcenter/               Notification center sidebar
    wifi/                       WiFi picker popup
    bluetooth/                  Bluetooth picker popup
    windowmgmt/                 Window management popup (grouped by workspace)
  controlpanel/                 Standalone settings UI (--control-panel / -c flag)
  lockscreen/                   Lock screen
  session/                      Power menu
  settings/                     Settings overlay
  mediaoverlay/                 Full-screen media controls
  notes/                        Notes overlay
  recorder/                     Screen recorder controls
  imageviewer/                  Floating image viewer
  polkit/                       PolicyKit authentication agent
  osd/                          Volume/brightness on-screen display
  notifpopup/                   Notification toast popups
  osk/                          On-screen keyboard (includes Emoji & Clipboard panels)
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
    audio/                      Volume control (native PulseAudio protocol over PipeWire socket)
    brightness/                 Brightness control (pure Go DDC/CI over I2C)
    resources/                  CPU/RAM monitoring (/proc, change detection skips <1% deltas)
    network/                    WiFi scanning + connectivity (NetworkManager DBus)
    bluetooth/                  Device discovery + pairing (Bluez DBus)
    mpris/                      Media player control (MPRIS2 DBus)
    upower/                     Battery status (UPower DBus)
    notifications/              Notification server (freedesktop DBus)
    clipboard/                  Clipboard watcher (cliphist integration)
    nightmode/                  Night light (hyprsunset)
    darkmode/                   Dark mode detection (xdg-desktop-portal / gsettings fallback)
    inputmode/                  Input mode switching (auto/tablet/desktop)
    idle/                       Idle/Timeout management (ext-idle-notify-v1 + D-Bus ScreenSaver)
    sni/                        System tray host (StatusNotifierItem DBus)
    runner/                     Command abstraction (Runner, StreamReader, Commander, PollLoop)
  ddc/                          Pure Go DDC/CI monitor control (I2C ioctls, bus caching)
  inputmethod/                  zwp_input_method_v2 watcher for OSK auto-show
  waylandutil/                  Shared Wayland helpers (fixed Bind encoding, roundtrips)
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
- **Event-driven architecture** — Services use event streams (PulseAudio protocol subscriptions, wl-paste --watch, MPRIS D-Bus signals, Hyprland socket events, `ext-idle-notify-v1`) instead of polling where possible.
- **Wayland Protocol Interop** — Built-in support for standard Wayland staging protocols (`ext-idle-notify-v1`, `zwp_input_method_v2`) using a custom `fixedBind` workaround to ensure high stability across different compositor versions.
- **Inhibition Support** — The Idle service natively respects compositor-level inhibitors (e.g., Firefox video playback) and provides an `org.freedesktop.ScreenSaver` D-Bus interface for legacy application compatibility.

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

## Building

```
make build
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

### Control commands

The binary accepts `--<command>` flags that send commands to the running instance via a Unix socket at `/tmp/snry-shell.sock`. This covers everything from surface toggles to hardware control — no external tools needed.

#### Surface toggles

| Flag | Effect |
|------|--------|
| `--toggle-overview` | Application overview |
| `--toggle-appdrawer` | Application drawer |
| `--toggle-calendar` | Calendar & Quick Settings |
| `--toggle-notif-center` | Notification center |
| `--toggle-wifi` | WiFi picker |
| `--toggle-bluetooth` | Bluetooth picker |
| `--toggle-windowmgmt` | Window management |
| `--toggle-session` | Power menu |
| `--toggle-crosshair` | Crosshair overlay |
| `--toggle-cheatsheet` | Keyboard shortcuts |
| `--toggle-media-overlay` | Full-screen media controls |
| `--toggle-settings` | Settings panel |
| `--toggle-region-selector` | Region screenshot selector |
| `--toggle-osk` | On-screen keyboard |
| `--toggle-clipboard` | Clipboard history (OSK panel) |
| `--toggle-emoji` | Emoji picker (OSK panel) |
| `--toggle-notes` | Notes overlay |
| `--toggle-recorder` | Screen recorder |
| `--toggle-reload-theme` | Force theme regeneration |
| `--toggle-lock` | Lock the screen |

#### Hardware & system control

These replace external CLI tools entirely — no `wpctl`, `brightnessctl`, `playerctl`, `loginctl`, or `systemctl` needed.

| Flag | Effect |
|------|--------|
| `--volume-up` | Raise default sink volume by 5% |
| `--volume-down` | Lower default sink volume by 5% |
| `--volume-mute` | Toggle default sink mute |
| `--mic-mute` | Toggle default source (mic) mute |
| `--brightness-up` | Raise display brightness by 5% (DDC or sysfs backlight) |
| `--brightness-down` | Lower display brightness by 5% |
| `--media-play-pause` | Play/pause the active MPRIS player |
| `--media-next` | Skip to next track |
| `--media-prev` | Previous track |
| `--zoom-in` | Increase Hyprland cursor zoom by 0.3 (clamped to 3.0) |
| `--zoom-out` | Decrease Hyprland cursor zoom by 0.3 (min 1.0) |
| `--zoom-reset` | Reset Hyprland cursor zoom to 1.0 |
| `--system-suspend` | Lock screen then suspend via logind D-Bus |
| `--system-reboot` | Reboot via logind D-Bus |
| `--system-poweroff` | Power off via logind D-Bus |
| `--system-logout` | Log out (Hyprland dispatch exit) |

#### Example Hyprland keybinds

```ini
# Volume
bindle = , XF86AudioRaiseVolume, exec, snry-shell --volume-up
bindle = , XF86AudioLowerVolume, exec, snry-shell --volume-down
bindl  = , XF86AudioMute,        exec, snry-shell --volume-mute
bindl  = , XF86AudioMicMute,     exec, snry-shell --mic-mute

# Brightness
bindle = , XF86MonBrightnessUp,   exec, snry-shell --brightness-up
bindle = , XF86MonBrightnessDown, exec, snry-shell --brightness-down

# Media
bindl = , XF86AudioPlay,  exec, snry-shell --media-play-pause
bindl = , XF86AudioNext,  exec, snry-shell --media-next
bindl = , XF86AudioPrev,  exec, snry-shell --media-prev

# Session
bindl = Super, L,         exec, snry-shell --toggle-lock
bindl = Super+Shift, L,   exec, snry-shell --system-suspend

# Zoom
binde = Super, Minus,     exec, snry-shell --zoom-out
binde = Super, Equal,     exec, snry-shell --zoom-in
bind  = Super, 0,         exec, snry-shell --zoom-reset
```

A complete example config is available at [`examples/hyprland.conf`](examples/hyprland.conf).

## Configuration

Settings are persisted in the key-value store at `~/.config/snry-shell/store.json`. The following keys are used:

| Key | Default | Description |
|-----|---------|-------------|
| `dark_mode` | `true` | Enable dark color scheme |
| `do_not_disturb` | `false` | Suppress notification toasts |
| `input_mode` | `"auto"` | Input mode: `"auto"`, `"tablet"`, or `"desktop"` |
| `idle_lock_timeout` | `300` | Seconds of inactivity before locking (0 = disabled) |
| `idle_displayoff_timeout` | `30` | Additional seconds after lock before display turns off (0 = disabled) |
| `idle_suspend_timeout` | `0` | Additional seconds after lock before suspend (0 = disabled) |
| `lock_max_attempts` | `3` | Max password attempts before lockout |
| `lockout_duration` | `30` | Seconds to lock out after max attempts |
| `lock_show_clock` | `true` | Show clock on lockscreen |
| `lock_show_user` | `true` | Show username on lockscreen |
| `theme.wallpaper` | `""` | Last detected wallpaper path (auto-updated) |

Settings can be changed from the built-in settings panel (`--toggle-settings`) or the standalone control panel (`--control-panel`).

## Testing

```
make test
```

Test suites cover the internal packages (bus, calendar, ddc, launcher, services, settings, theme, uinput, controlsocket, osk).

## License

[MIT](LICENSE)
