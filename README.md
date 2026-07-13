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
│   ├── visualizer.go # Audio visualizer component
│   └── visualizer_test.go
├── mpris/
│   ├── client.go     # MPRIS D-Bus client for Spotify control
│   └── client_test.go
├── lyrics/
│   ├── engine.go     # Lyrics fetching and sync (LRCLIB)
│   └── engine_test.go
└── README.md
```

## How It Works

The TUI connects to Spotify via the [MPRIS](https://specifications.freedesktop.org/mpris-spec/latest/) (Media Player Remote Interfacing Specification) D-Bus interface. This allows full control of playback without requiring Spotify API credentials.

Lyrics are fetched from [LRCLIB](https://lrclib.net/), a free and open-source lyrics database.

## License

MIT License - see [LICENSE](LICENSE) for details.
