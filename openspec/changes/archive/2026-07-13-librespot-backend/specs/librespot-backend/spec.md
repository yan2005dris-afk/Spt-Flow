# Librespot Backend Specification

## Purpose

Replace the Spotify desktop app dependency with `librespot` as a headless Spotify Connect backend. The TUI launches and controls `librespot` via MPRIS, providing identical functionality to Spotify desktop without requiring the GUI app.

---

## ADDED Requirements

### Requirement: LaunchLibrespot

`mpris.Client` SHALL provide `LaunchLibrespot(configPath string) error` that:
1. Checks `librespot` binary exists via `exec.Lookup("librespot")` — returns `ErrLibrespotNotInstalled` if not found
2. Starts `librespot --config <configPath> --enable-oauth --name "Spt-Flow"` as a subprocess with `SysProcAttr{Setpgid: true}`
3. Stores the `*exec.Cmd` via `SetSpotifyCmd()`
4. Returns `nil` on successful start

The config path defaults to `~/.config/librespot.conf` or `~/.config/tui-spotify/librespot.conf`.

### Requirement: IsSpotifyDesktopRunning

`mpris.Client` SHALL provide `IsSpotifyDesktopRunning() bool` that:
1. Calls `GetNameOwner()` for `org.mpris.MediaPlayer2.spotify`
2. Returns `true` if the owner is `org.mpris.MediaPlayer2.spotify` (not `librespot`)
3. Returns `false` otherwise

### Requirement: New Error Types

`mpris/client.go` SHALL define:
- `ErrLibrespotNotInstalled = errors.New("librespot not found in PATH")`
- `ErrSpotifyDesktopRunning = errors.New("Spotify desktop is running — please close it first")`

### Requirement: --setup Flag

When `--setup` is passed, `main.go` SHALL:
1. Check `exec.Lookup("librespot")` — if not found, print install instructions and exit with code 1
2. Launch `librespot --enable-oauth --name "Spt-Flow" --cache ~/.cache/librespot`
3. Capture stdout — parse OAuth URL (format: "Go to: https://...")
4. Display the URL and prompt user to complete auth in browser
5. Poll `IsRunning()` until true or 60s timeout
6. Print success message and exit (one-time setup)

### Requirement: Menu Option 1 — Librespot

Menu option 1 SHALL now be labeled `"Start with Librespot + TUI"` and SHALL:
1. Call `IsSpotifyDesktopRunning()` — if `true`, return error "Spotify desktop is running"
2. Call `LaunchLibrespot(configPath)` — if `ErrLibrespotNotInstalled`, show install instructions
3. If launch succeeds, set `LaunchedSpotify = true`
4. Transition to TUI view

### Requirement: Menu Option 5 — Spotify Desktop

The menu SHALL add a fifth option labeled `"Start with Spotify Desktop"` that:
1. Calls `IsSpotifyDesktopRunning()` — if `true`, skips launch (already running)
2. Calls `LaunchSpotify()` via D-Bus `StartServiceByName`
3. Sets `LaunchedSpotify = true`
4. Transitions to TUI view

Help renumbers to option 6.

### Requirement: pollSpotifyCmd — No Auto-Launch

`pollSpotifyCmd()` SHALL NOT call `LaunchSpotify()` or any launch mechanism when `!IsRunning()`. It SHALL return `SpotifyStateMsg{Running: false}` and let the menu handle launching.

### Requirement: Kill on Exit — Librespot

When the TUI exits after launching librespot, `KillSpotify()` SHALL send SIGTERM to the process group created by `Setpgid: true`, terminating only the process the TUI launched.

---

## Scenarios

### Scenario: User runs --setup with librespot not installed

- GIVEN `--setup` flag is passed
- WHEN `exec.Lookup("librespot")` fails
- THEN print install instructions to stdout and exit with code 1

### Scenario: User runs --setup successfully

- GIVEN `--setup` flag is passed and `librespot` is installed
- WHEN the OAuth URL is parsed from stdout
- THEN display "Go to: https://..." and prompt user to complete auth
- WHEN `IsRunning()` returns true within 60s
- THEN print "Librespot connected successfully" and exit with code 0

### Scenario: Menu option 1 — Spotify desktop running

- GIVEN Spotify desktop is running (`IsSpotifyDesktopRunning() == true`)
- WHEN user selects menu option 1
- THEN show error "Spotify desktop is running — please close it first"
- AND stay in menu

### Scenario: Menu option 1 — librespot not installed

- GIVEN Spotify desktop is not running
- WHEN user selects menu option 1 and `LaunchLibrespot` returns `ErrLibrespotNotInstalled`
- THEN show install instructions
- AND stay in menu

### Scenario: Menu option 1 — librespot launches successfully

- GIVEN Spotify desktop is not running and `librespot` is installed
- WHEN user selects menu option 1
- THEN `LaunchLibrespot()` starts the process
- AND `LaunchedSpotify = true`
- AND TUI transitions to player view

### Scenario: Menu option 5 — Spotify desktop launched

- GIVEN Spotify desktop is not running
- WHEN user selects menu option 5
- THEN `LaunchSpotify()` calls D-Bus StartServiceByName
- AND `LaunchedSpotify = true`
- AND TUI transitions to player view

### Scenario: TUI closed after launching librespot

- GIVEN TUI launched `librespot` (`LaunchedSpotify = true`, `spotifyCmd != nil`)
- WHEN user presses Ctrl+C or sends SIGTERM
- THEN `KillSpotify()` sends SIGTERM to the process group
- AND `librespot` terminates

### Scenario: TUI closed after launching Spotify desktop

- GIVEN TUI launched Spotify desktop (`LaunchedSpotify = true`, `spotifyCmd != nil`)
- WHEN user presses Ctrl+C or sends SIGTERM
- THEN `KillSpotify()` sends SIGTERM to the process group
- AND Spotify desktop terminates

### Scenario: TUI closed without launching any player

- GIVEN `LaunchedSpotify = false` and `spotifyCmd = nil`
- WHEN TUI exits
- THEN no kill is attempted
