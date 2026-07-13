# Spotify Lifecycle Specification

## Purpose

Give the TUI the ability to auto-launch Spotify when missing and cleanly terminate it on exit — but only when the TUI itself started the process.

---

## ADDED Requirements

### Requirement: LaunchSpotify via D-Bus (Secondary)

The system SHALL provide `LaunchSpotify() error` on `mpris.Client` that calls `org.freedesktop.DBus.StartServiceByName("org.mpris.MediaPlayer2.spotify", 0)` on the bus object and returns an error if the call fails.

**This is now the secondary launch path** — used only when menu option 5 ("Start with Spotify Desktop") is selected. The primary path (menu option 1) launches `librespot` instead.

### Requirement: Flatpak/Snap detection as fatal

The system MUST detect when Spotify is running inside a Flatpak or Snap sandbox. If `os.Getenv("FLATPAK_ID") != ""` AND `IsRunning()` returns false after `LaunchSpotify()`, `LaunchSpotify` MUST return `ErrFlatpakSandbox`. The shell MUST treat this as a fatal error: set `m.ErrorMessage = "Spotify is running as Flatpak/Snap — not compatible with this TUI"` and stop polling.

### Requirement: KillSpotify via SIGTERM

`mpris.Client` MUST provide `KillSpotify() error` sending `syscall.SIGTERM` to stored `*exec.Cmd.Process`. Uses process group if `Setpgid` was set. This is called only when `spotifyCmd != nil` (i.e., only when we launched it from menu option 1 or 5).

### Requirement: LaunchLibrespot as Primary

`mpris.Client` SHALL provide `LaunchLibrespot(configPath string) error` as the primary launch mechanism. This MUST be called before `LaunchSpotify()` when menu option 1 is selected.

`LaunchLibrespot` SHALL:
1. Check `librespot` binary exists via `exec.Lookup("librespot")` — return `ErrLibrespotNotInstalled` if not found
2. Call `IsSpotifyDesktopRunning()` — return `ErrSpotifyDesktopRunning` if Spotify desktop is active
3. Start `librespot --config <configPath> --enable-oauth --name "Spt-Flow"` via `exec.Command().Start()` with `SysProcAttr{Setpgid: true}`
4. Store the `*exec.Cmd` via `SetSpotifyCmd()`

### Requirement: IsSpotifyDesktopRunning Detection

`mpris.Client` SHALL provide `IsSpotifyDesktopRunning() bool` that:
1. Calls `GetNameOwner()` for `org.mpris.MediaPlayer2.spotify`
2. Returns `true` if the owner is `org.mpris.MediaPlayer2.spotify` (not `librespot`)
3. Returns `false` otherwise

This MUST be called before launching `librespot` to prevent conflicts.

### Requirement: New Error Types for Librespot

`mpris/client.go` SHALL define these errors for librespot integration:
- `ErrLibrespotNotInstalled = errors.New("librespot not found in PATH")`
- `ErrSpotifyDesktopRunning = errors.New("Spotify desktop is running — please close it first")`

`ErrLibrespotNotInstalled` is returned by `LaunchLibrespot()` when the binary is not in PATH.
`ErrSpotifyDesktopRunning` is returned when Spotify desktop is detected before trying to launch `librespot`.

### Requirement: Store spotifyCmd on Client

`mpris.Client` struct stores `spotifyCmd *exec.Cmd` (nil if not launched by TUI). Menu option 1 stores the `*exec.Cmd` returned by `LaunchLibrespot()`. Menu option 5 stores the `*exec.Cmd` from `exec.Command("spotify").Start()`.

**This applies to both Spotify desktop and librespot** — `KillSpotify()` uses the same kill mechanism regardless of which player was launched.

### Requirement: LaunchedSpotify field on Model

`ui.Model` has `LaunchedSpotify bool` field. It is set to `true` only when menu option 1 is selected and Spotify is launched. It is set to `false` on options 2, 3, and `--no-menu`. Menu option 3 MUST NOT set `LaunchedSpotify = true`.

### Requirement: Kill on signal in main.go

On `SIGINT`/`SIGTERM`, if `mprisClient.spotifyCmd != nil`, call `KillSpotify()` before exiting. Use `signal.Notify` in a goroutine with `defer` cleanup as fallback.

**Auto-launch now happens from menu option 1, NOT from main.go or pollSpotifyCmd.** `main.go` MUST NOT pre-launch Spotify — it starts the program in `"menu"` state. `pollSpotifyCmd` MUST NOT call `LaunchSpotify()`.

### Requirement: pollSpotifyCmd Does Not Launch

`pollSpotifyCmd()` in `ui/shell.go` MUST NOT call `LaunchSpotify()` or `LaunchLibrespot()` when `!IsRunning()`. The polling loop only connects to an already-running player. Launching is the menu's responsibility.

---

## Scenarios

### Scenario: Spotify is Flatpak — incompatibility error and exit

- GIVEN `FLATPAK_ID` env var is set
- WHEN `LaunchSpotify()` is called and Spotify is not already running
- THEN `LaunchSpotify` returns `ErrFlatpakSandbox`
- AND the TUI sets `m.ErrorMessage = "Spotify is running as Flatpak/Snap — not compatible with this TUI"`
- AND the TUI exits

### Scenario: Pre-existing Spotify — no kill on exit

- GIVEN Spotify was already running before the TUI launched
- WHEN the user closes the TUI
- THEN `spotifyCmd` is nil
- AND Spotify process is NOT killed

### Scenario: TUI-launched Spotify — killed on exit

- GIVEN the TUI launched Spotify itself (`LaunchedSpotify = true`)
- WHEN the user presses Ctrl+C or sends SIGTERM
- THEN `KillSpotify()` is called
- AND Spotify process receives SIGTERM
- AND Spotify terminates

### Scenario: Spotify crashes after being launched

- GIVEN the TUI launched Spotify and `spotifyCmd != nil`
- WHEN Spotify crashes and `IsRunning()` returns false
- THEN the TUI shows "Waiting..." state
- AND on TUI exit, no kill is attempted (process is already gone)

### Scenario: Option 1 → Spotify launched, killed on exit

- GIVEN Spotify not running
- WHEN menu option 1 selected
- THEN `LaunchedSpotify = true`, Spotify starts
- AND on exit `KillSpotify()` called

### Scenario: Option 2 → TUI only, no kill

- GIVEN any state
- WHEN menu option 2 selected
- THEN `LaunchedSpotify = false`, Spotify NOT killed on exit

### Scenario: Option 3 → status only, no kill

- GIVEN any state
- WHEN menu option 3 selected
- THEN `LaunchedSpotify = false`, status shown, Spotify NOT killed on exit

### Scenario: Pre-existing Spotify, option 2

- GIVEN Spotify already running
- WHEN menu option 2 selected
- THEN `spotifyCmd = nil`, `LaunchedSpotify = false`, Spotify NOT killed on exit

### Scenario: --no-menu → TUI only

- GIVEN `--no-menu` flag
- WHEN Init runs
- THEN `LaunchedSpotify = false`, no pre-launch, Spotify not killed on exit

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
