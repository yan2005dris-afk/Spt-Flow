# Spotify TUI Mirror (`tui-spotify`)

[![Go Version](https://img.shields.io/badge/Go-1.26-blue)](https://tip.golang.org/doc/go1.26)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A beautiful, lightweight Terminal User Interface (TUI) that mirrors and controls your local Spotify desktop application on Linux. It respects your custom themes (like Spicetify) and operates entirely offline via MPRIS D-Bus — meaning **no Spotify API credentials or login required**.

## Screenshots

> _Add your terminal screenshots here_

```
┌─────────────────────────────────────────────────────────────┐
│  🎵 Now Playing                                           │
│  Artist - Track Title                                      │
│  ░░░░░░░░░░░░░░░░░░░░░░░  67%  🔊 80%                    │
│                                                             │
│  ♪ ♫ ♪ ♫ ♪ ♫ ♪ ♫ ♪ ♫ ♪ ♫ ♪ ♫ ♪ ♫ ♪ ♫ ♪ ♫ ♪ ♫            │
│                                                             │
│  Lyrics would appear here synced with playback...          │
└─────────────────────────────────────────────────────────────┘
```

## Features

- **Zero Configuration**: Connects instantly to your running desktop Spotify instance.
- **Synced Scrolling Lyrics**: Fetches synced lyrics dynamically from LRCLIB and scrolls them in perfect sync with the playback position.
- **Plain Text Fallback**: Automatically falls back to unsynced plain text lyrics with manual scroll controls if synced lyrics are unavailable.
- **Procedural Visualizer**: A gorgeous, lightweight audio visualizer built from block characters that responds to play, pause, and track status.
- **Responsive Layout**: Adapts gracefully to your terminal size. The visualizer is collapsed on widths under 80 columns to prioritize lyrics.
- **Full Playback Controls**: Control track states, skips, and volume directly from your keyboard.

## Keybindings

| Key | Action |
|---|---|
| `Space` | Play / Pause |
| `n` or `l` | Next track |
| `p` or `h` | Previous track |
| `+` or `=` | Increase volume (10% increments) |
| `-` | Decrease volume (10% decrements) |
| `j` or `↓` | Scroll down plain-text lyrics |
| `k` or `↑` | Scroll up plain-text lyrics |
| `q` or `Ctrl+C` | Quit the TUI |

## Prerequisites

- **OS**: Linux (with a running D-Bus session).
- **Player**: Desktop Spotify client running locally.
- **Language**: Go 1.21+ (built with Bubble Tea & Lip Gloss).

## Installation

### Build from source

```bash
git clone https://github.com/yourusername/Spt-Flow.git
cd Spt-Flow
go build -o tui-spotify
```

### Run directly

```bash
go run .
```

## Development

```bash
# Run tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Project Structure

```
Spt-Flow/
├── main.go           # Application entry point
├── ui/
│   ├── shell.go      # TUI shell and layout
│   ├── shell_test.go
│   ├── menu.go       # Startup menu model and rendering
│   ├── theme.go      # Theme struct + DefaultTheme palette
│   ├── themes.go     # Palette registry (Themes, ThemeOrder, CatppuccinMocha, GruvboxDark)
│   ├── themes_test.go
│   ├── config.go     # Config load/save (XDG_CONFIG_HOME/spt-flow/config.json)
│   ├── config_test.go
│   ├── visualizer.go # Audio visualizer component
│   └── visualizer_test.go
├── mpris/
│   ├── client.go     # MPRIS D-Bus client for Spotify control
│   └── client_test.go
├── lyrics/
│   ├── engine.go     # Lyrics fetching and sync (LRCLIB)
│   ├── engine_test.go
│   └── cache/
│       ├── cache.go      # Lyrics cache (LRU, 7-day TTL)
│       └── cache_test.go
└── README.md
```

## How It Works

The TUI connects to Spotify via the [MPRIS](https://specifications.freedesktop.org/mpris-spec/latest/) (Media Player Remote Interfacing Specification) D-Bus interface. This allows full control of playback without requiring Spotify API credentials.

Lyrics are fetched from [LRCLIB](https://lrclib.net/), a free and open-source lyrics database.

## Lyrics Cache

Lyrics are cached locally to avoid repeated network requests for the same track. The cache is stored at:

- **Location**: `~/.cache/spt-flow/lyrics.json` (or `$XDG_CACHE_HOME/spt-flow/lyrics.json`)
- **TTL**: 7 days
- **Capacity**: 200 entries (LRU eviction when exceeded)

To clear the cache manually:

```bash
rm ~/.cache/spt-flow/lyrics.json
```

## Themes

Spt-Flow supports multiple color palettes. You can preview and switch themes directly from the startup menu.

### Available Palettes

| Name | Description |
|------|-------------|
| `default` | Classic terminal colors (ANSI 16-color) |
| `catppuccin-mocha` | Warm mauve/teal palette from the Catppuccin community |
| `gruvbox-dark` | Retro earthy tones from the Gruvbox project |

### Switching Themes

1. Run `tui-spotify`
2. From the startup menu, navigate to **Theme: \<name\>** and press **Enter**
3. The menu re-renders with the new palette immediately
4. Your choice is persisted to `~/.config/spt-flow/config.json`

The config file is created automatically with mode `0700`. On restart, the saved theme is restored automatically. If the config file is missing, corrupt, or contains an unknown theme name, the TUI falls back to `default` without any user-visible error.

### Config File Location

```text
~/.config/spt-flow/config.json   (Linux, XDG_CONFIG_HOME fallback)
$XDG_CONFIG_HOME/spt-flow/config.json
```

The file format:
```json
{ "theme": "catppuccin-mocha" }
```

## License

MIT License - see [LICENSE](LICENSE) for details.
