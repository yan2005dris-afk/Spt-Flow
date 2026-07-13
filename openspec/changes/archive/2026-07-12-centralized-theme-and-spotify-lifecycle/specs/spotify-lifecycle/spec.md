# Spotify Lifecycle Specification

## Purpose

Give the TUI the ability to auto-launch Spotify when missing and cleanly terminate it on exit — but only when the TUI itself started the process.

---

## ADDED Requirements

### Requirement: LaunchSpotify via D-Bus

The system SHALL provide `LaunchSpotify() error` on `mpris.Client` that calls `org.freedesktop.DBus.StartServiceByName("org.mpris.MediaPlayer2.spotify", 0)` on the bus object and returns an error if the call fails.

### Requirement: Flatpak/Snap detection as fatal

The system MUST detect when Spotify is running inside a Flatpak or Snap sandbox. If `os.Getenv("FLATPAK_ID") != ""` AND `IsRunning()` returns false after `LaunchSpotify()`, `LaunchSpotify` MUST return `ErrFlatpakSandbox`. The shell MUST treat this as a fatal error: set `m.ErrorMessage = "Spotify is running as Flatpak/Snap — not compatible with this TUI"` and stop polling.

### Requirement: KillSpotify via SIGTERM

The system SHALL provide `KillSpotify() error` on `mpris.Client` that sends `syscall.SIGTERM` to Spotify's stored `*exec.Cmd.Process`. If the process was launched with `Setpgid: true`, the signal is sent to the process group.

### Requirement: Store spotifyCmd on Client

The `mpris.Client` struct MUST store `spotifyCmd *exec.Cmd` (nil if not launched by the TUI).

### Requirement: LaunchedSpotify field on Model

The `ui.Model` struct SHALL have a `LaunchedSpotify bool` field.

### Requirement: Auto-launch in pollSpotifyCmd

When `!IsRunning()` and `!m.LaunchedSpotify`, the `pollSpotifyCmd()` function MUST call `LaunchSpotify()`. On success, it MUST set `m.LaunchedSpotify = true`. If `LaunchSpotify` returns `ErrFlatpakSandbox`, the function MUST set `m.ErrorMessage` to the Flatpak incompatibility message and set `m.SpotifyRunning = false` to stop polling.

### Requirement: Retry limit with permanent error

The `Model` SHALL track `launchRetries int`. After 3 consecutive failed `LaunchSpotify()` attempts, the system MUST set a permanent error `"Failed to launch Spotify after 3 attempts"` and stop retrying.

### Requirement: Kill on signal in main.go

On `SIGINT` or `SIGTERM`, if `mprisClient != nil && mprisClient.spotifyCmd != nil`, the system MUST call `mprisClient.KillSpotify()` before exiting. The signal handler runs in a goroutine with `signal.Notify(osSigCh, syscall.SIGINT, syscall.SIGTERM)`. A `defer` cleanup MUST serve as fallback.

---

## Scenarios

### Scenario: Spotify auto-launched via D-Bus

- GIVEN Spotify is not running
- WHEN the TUI starts and `pollSpotifyCmd` fires
- THEN `LaunchSpotify()` is called via D-Bus
- AND `m.LaunchedSpotify` is set to `true`
- AND Spotify appears in the TUI within 2–8 seconds

### Scenario: Spotify not installed — permanent error after retries

- GIVEN Spotify is not installed
- WHEN `LaunchSpotify()` fails 3 consecutive times
- THEN the TUI displays `"Failed to launch Spotify after 3 attempts"`
- AND polling stops permanently

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
