# Spotify Lifecycle Specification (Delta)

## MODIFIED Requirements

### Requirement: Kill on signal in main.go

(Previously: Spotify auto-launch in pollSpotifyCmd; main.go launched Spotify before TUI)

On `SIGINT`/`SIGTERM`, if `mprisClient.spotifyCmd != nil`, call `KillSpotify()` before exiting. Use `signal.Notify` in a goroutine with `defer` cleanup as fallback.

**Auto-launch now happens from menu option 1, NOT from main.go or pollSpotifyCmd.** `main.go` MUST NOT pre-launch Spotify — it starts the program in `"menu"` state. `pollSpotifyCmd` MUST NOT call `LaunchSpotify()`.

### Requirement: LaunchedSpotify field on Model

(Previously: `LaunchedSpotify` set true when auto-launch succeeded in pollSpotifyCmd)

`ui.Model` has `LaunchedSpotify bool` field. It is set to `true` only when menu option 1 is selected and Spotify is launched. It is set to `false` on options 2, 3, and `--no-menu`. Menu option 3 MUST NOT set `LaunchedSpotify = true`.

### Requirement: KillSpotify via SIGTERM

`mpris.Client` MUST provide `KillSpotify() error` sending `syscall.SIGTERM` to stored `*exec.Cmd.Process`. Uses process group if `Setpgid` was set. This is called only when `spotifyCmd != nil` (i.e., only when we launched it from menu option 1).

### Requirement: Store spotifyCmd on Client

`mpris.Client` struct stores `spotifyCmd *exec.Cmd` (nil if not launched by TUI). Menu option 1 stores the `*exec.Cmd` returned by `exec.Command("spotify").Start()` with `SysProcAttr{Setpgid: true}`.

## REMOVED Requirements

### Requirement: Auto-launch in pollSpotifyCmd

(Reason: Auto-launch moved to menu option 1. `pollSpotifyCmd` no longer calls `LaunchSpotify()`. The TUI waits indefinitely if Spotify is not running and was not launched.)

### Requirement: Retry limit with permanent error

(Reason: Retry logic was tied to auto-launch flow. With menu-driven launch, retry is not needed — user chooses whether to launch.)

## Scenarios

1. **Option 1 → Spotify launched, killed on exit** — GIVEN Spotify not running → WHEN menu option 1 selected → THEN `LaunchedSpotify = true`, Spotify starts, on exit `KillSpotify()` called.
2. **Option 2 → TUI only, no kill** — GIVEN any state → WHEN menu option 2 selected → THEN `LaunchedSpotify = false`, Spotify NOT killed on exit.
3. **Option 3 → status only, no kill** — GIVEN any state → WHEN menu option 3 selected → THEN `LaunchedSpotify = false`, status shown, Spotify NOT killed on exit.
4. **Pre-existing Spotify, option 2** — GIVEN Spotify already running → WHEN menu option 2 selected → THEN `spotifyCmd = nil`, `LaunchedSpotify = false`, Spotify NOT killed on exit.
5. **--no-menu → TUI only** — GIVEN `--no-menu` flag → WHEN Init runs → THEN `LaunchedSpotify = false`, no pre-launch, Spotify not killed on exit.
