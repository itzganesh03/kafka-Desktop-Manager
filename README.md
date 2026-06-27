# Kafka Desktop Manager

A modern Windows desktop application (Go + [Fyne](https://fyne.io)) that replaces
manual terminal usage for Apache Kafka. Start/stop ZooKeeper and the broker,
manage topics, produce and consume messages, inspect consumer groups, and watch
live logs — all from one graphical interface, similar in spirit to Docker
Desktop or MongoDB Compass.

## Features

- **First-run setup wizard** — point it at your Kafka install; it validates the
  required `.bat` scripts and config files, then saves your configuration.
- **Dashboard** — live status cards for the install, ZooKeeper, the broker,
  bootstrap address, topic count and consumer-group count (auto-refreshing).
- **Services & Logs** — Start / Stop / Restart ZooKeeper and the Kafka broker
  with per-service live log consoles (export & clear supported).
- **Topics** — searchable table (name, partitions, replication), plus Create,
  Describe, Delete, and a Utilities menu (Purge, Recreate, Sample messages,
  Reset offsets).
- **Producer** — topic selector, multi-line editor, JSON formatter, send single
  or per-line messages, send history.
- **Consumer** — start/stop/pause/resume, live message stream, search filter,
  auto-scroll toggle, copy-all and export.
- **Consumer Groups** — list groups with state & members; describe shows
  current offset, log-end offset and lag (lag highlighted in red).
- **Command History** — every executed Kafka command is recorded; copy or
  re-run with one click.
- **Settings** — Kafka path, bootstrap server, ZooKeeper port, default topic,
  auto-start toggles, theme (dark/light), open Kafka/log folders.
- Dark theme, toast notifications, Windows notifications, keyboard shortcuts.

## Requirements

- Windows 10/11
- A local Apache Kafka installation (the classic ZooKeeper-based distribution
  with `bin\windows\*.bat` scripts)

### Build requirements

- **Go 1.24+**
- A C compiler for CGO (Fyne requires it). This project was built with
  **mingw-w64 (UCRT) gcc** from MSYS2:
  ```
  winget install -e --id MSYS2.MSYS2
  C:\msys64\usr\bin\pacman.exe -S --noconfirm mingw-w64-ucrt-x86_64-gcc
  ```

## Assets (icon & splash)

The app icon is generated from code (no external image tools needed):

```powershell
# Regenerates internal/ui/icon.png (window/splash) and icon.ico (exe)
go run ./tools/genicon

# Embeds icon.ico into the .exe (creates icon_windows.syso, auto-linked by go build)
go run github.com/akavel/rsrc@latest -ico icon.ico -arch amd64 -o icon_windows.syso
```

On launch, an animated **"One Way Kafka Manager"** splash screen shows for ~5
seconds (pulsing logo, fading title, flowing dots, progress bar) before the main
window appears.

## Building

```powershell
# Ensure the compiler is on PATH and CGO is enabled
$env:PATH = "C:\msys64\ucrt64\bin;$env:PATH"
$env:CGO_ENABLED = "1"

# -H windowsgui makes it a GUI app so no console window appears at launch
go build -ldflags="-H windowsgui" -o kafka-desktop.exe .
```

> The first build is slow (3-6 min) because Fyne's C dependencies (GLFW/OpenGL)
> are compiled from scratch. Subsequent builds are cached and fast.

Run with:

```powershell
.\kafka-desktop.exe
```

## Configuration

Settings are stored locally at:

```
%APPDATA%\KafkaDesktopManager\config.json
```

Example:

```json
{
    "kafka_path": "C:\\kafka",
    "bootstrap_server": "localhost:9092",
    "zookeeper_port": "2181",
    "default_topic": "",
    "auto_start_zookeeper": false,
    "auto_start_kafka": false,
    "theme": "dark"
}
```

## Project layout

```
main.go                      entry point
internal/
  config/      config.go     load/save JSON config + install validation
  kafka/                     Kafka command wrappers (the backend)
    runner.go                command execution + history recording
    service.go               ZooKeeper/broker process management + log streaming
    topics.go                list/create/describe/delete topics
    producer.go              console producer
    consumer.go              streaming console consumer
    groups.go                consumer groups + offsets/lag
    utilities.go             purge/recreate/sample messages
    history.go               command history store
    util.go / proc_windows.go  helpers (process tree kill, hidden windows)
  ui/                        Fyne desktop UI
    app.go                   window shell, navigation, shortcuts
    theme.go                 dark/light themes
    wizard.go                first-run setup
    dashboard.go             live status cards
    services.go              service controls + log consoles
    topics.go                topic management
    producer.go              producer page
    consumer.go              consumer page
    groups.go                consumer groups page
    history.go               command history page
    settings.go              settings page
    helpers.go               cards, toasts, dialogs, export
```

## Keyboard shortcuts

- `Ctrl+1`..`Ctrl+8` — switch between pages
- `F5` — refresh the current page

## Notes

- The app drives the standard Kafka `.bat` tools under `bin\windows`; it does not
  embed a Kafka client library, so behaviour matches running the commands by hand.
- Stopping a service kills the whole process tree (the `.bat` launches a `java`
  child), so the broker/ZooKeeper actually stop.
- **Purge** is implemented as delete + recreate (preserving partitions and
  replication factor), the most reliable purge using only the bundled tooling.
