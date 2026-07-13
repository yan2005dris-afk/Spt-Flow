# Delta for Spotify Lifecycle

## Purpose

Update the Spotify lifecycle spec to add librespot as the preferred launch mechanism, while preserving Spotify desktop as an option.

---

## MODIFIED Requirements

### Requirement: LaunchSpotify via D-Bus

(Previously: LaunchSpotify via D-Bus as the primary mechanism)

`mpris.Client` SHALL provide `LaunchSpotify() error` that calls `org.freedesktop.DBus.StartServiceByName("org.mpris.MediaPlayer2.spotify", 0)` on the bus object and returns an error if the call fails.

**This is now the secondary launch path** — used only when menu option 5 ("Start with Spotify Desktop") is selected. The primary path (menu option 1) launches `librespot` instead.

### Requirement: LaunchLibrespot as Primary

(Previously: N/A — new behavior)

`mpris.Client` SHALL provide `LaunchLibrespot(configPath string) error` as the primary launch mechanism. This MUST be called before `LaunchSpotify()` when menu option 1 is selected.

`LaunchLibrespot` SHALL:
1. Check `librespot` binary exists via `exec.Lookup("librespot")` — return `ErrLibrespotNotInstalled` if not found
2. Call `IsSpotifyDesktopRunning()` — return `ErrSpotifyDesktopRunning` if Spotify desktop is active
3. Start `librespot --config <configPath> --enable-oauth --name "Spt-Flow"` via `exec.Command().Start()` with `SysProcAttr{Setpgid: true}`
4. Store the `*exec.Cmd` via `SetSpotifyCmd()`

### Requirement: IsSpotifyDesktopRunning Detection

(Previously: N/A — new behavior)

`mpris.Client` SHALL provide `IsSpotifyDesktopRunning() bool` that:
1. Calls `GetNameOwner()` for `org.mpris.MediaPlayer2.spotify`
2. Returns `true` if the owner is `org.mpris.MediaPlayer2.spotify` (not `librespot`)
3. Returns `false` otherwise

This MUST be called before launching `librespot` to prevent conflicts.

### Requirement: Store spotifyCmd on Client

(Previously: Store spotifyCmd on Client for Spotify desktop)

`mpris.Client` struct stores `spotifyCmd *exec.Cmd` (nil if not launched by TUI). Menu option 1 stores the `*exec.Cmd` returned by `LaunchLibrespot()`. Menu option 5 stores the `*exec.Cmd` from `exec.Command("spotify").Start()`.

**This applies to both Spotify desktop and librespot** — `KillSpotify()` uses the same kill mechanism regardless of which player was launched.

---

## ADDED Requirements

### Requirement: New Error Types

`mpris/client.go` SHALL define these errors for librespot integration:
- `ErrLibrespotNotInstalled = errors.New("librespot not found in PATH")`
- `ErrSpotifyDesktopRunning = errors.New("Spotify desktop is running — please close it first")`

`ErrLibrespotNotInstalled` is returned by `LaunchLibrespot()` when the binary is not in PATH.
`ErrSpotifyDesktopRunning` is returned when Spotify desktop is detected before trying to launch `librespot`.

### Requirement: pollSpotifyCmd Does Not Launch

`pollSpotifyCmd()` in `ui/shell.go` MUST NOT call `LaunchSpotify()` or `LaunchLibrespot()` when `!IsRunning()`. The polling loop only connects to an already-running player. Launching is the menu's responsibility.

---

## REMOVED Requirements

### Requirement: LaunchSpotify via D-Bus as Primary

(Reason: `librespot` via `LaunchLibrespot()` is now the preferred mechanism. D-Bus StartServiceByName remains available via menu option 5 for users who prefer the desktop app.)
(Migration: Menu option 1 now calls `LaunchLibrespot()`; option 5 calls `LaunchSpotify()`)

---

## Scenarios (Updated)

### Scenario: Spotify desktop running, user selects option 1

- GIVEN Spotify desktop is running
- WHEN user selects menu option 1 (Start with Librespot)
- THEN `IsSpotifyDesktopRunning()` returns `true`
- AND the TUI shows error "Spotify desktop is running — please close it first"
- AND remains in menu

### Scenario: Librespot not installed, user selects option 1

- GIVEN `librespot` is not in PATH
- WHEN user selects menu option 1
- THEN `LaunchLibrespot()` returns `ErrLibrespotNotInstalled`
- AND the TUI shows install instructions
- AND remains in menu

### Scenario: Librespot launches successfully via menu option 1

- GIVEN Spotify desktop is not running and `librespot` is installed
- WHEN user selects menu option 1
- THEN `LaunchLibrespot()` starts the process
- AND `spotifyCmd` is set via `SetSpotifyCmd()`
- AND `LaunchedSpotify = true`
- AND TUI transitions to player view

### Scenario: Spotify desktop launched via menu option 5

- GIVEN Spotify desktop is not running
- WHEN user selects menu option 5 (Start with Spotify Desktop)
- THEN `LaunchSpotify()` calls D-Bus StartServiceByName
- AND `spotifyCmd` is set via `SetSpotifyCmd()`
- AND `LaunchedSpotify = true`
- AND TUI transitions to player view

### Scenario: TUI kills only the player it launched

- GIVEN TUI launched `librespot` via option 1 OR Spotify desktop via option 5
- WHEN TUI exits (Ctrl+C, SIGTERM, or 'q')
- THEN `KillSpotify()` sends SIGTERM to the stored `spotifyCmd` process group
- AND only the launched player is terminated

### Scenario: Spotify crashes after being launched

- GIVEN TUI launched a player and `spotifyCmd != nil`
- WHEN the player crashes and `IsRunning()` returns false
- THEN TUI shows "Waiting..." state
- AND on TUI exit, no kill is attempted (process is already gone)
