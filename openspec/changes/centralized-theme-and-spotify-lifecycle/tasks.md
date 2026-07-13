# Tasks: Centralized Theme & Spotify Lifecycle

## Review Workload Forecast

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: single-pr
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Implement centralized Theme + Spotify lifecycle | PR 1 | `go test ./...` | Manual: run TUI, verify Spotify pre-launch, test SIGINT cleanup | Revert 5 files |

## Phase 1: Foundation (ui/theme.go)

- [x] 1.1 Create `ui/theme.go` with `Theme` struct (8 fields) and `DefaultTheme` package var
  - Fields: Header, Footer, LyricActive, LyricInactive, LyricPlain, Visualizer, Error, Waiting
  - All colors as `lipgloss.ANSIColor(N)` matching design spec
  - Verification: `go build ./ui/...`

## Phase 2: Core Implementation (mpris/client.go)

- [x] 2.1 Add `ErrFlatpakSandbox` error var to `mpris/client.go`
- [x] 2.2 Add `spotifyCmd *exec.Cmd` field to `Client` struct
- [x] 2.3 Add `LaunchSpotify() error` method:
  - Check BOTH `FLATPAK_ID` AND `SNAP_NAME` env vars (per user requirement)
  - If either is set and Spotify isn't running, return `ErrFlatpakSandbox`
  - Call D-Bus `StartServiceByName` for Spotify
  - NO exec.Command fallback inside this method (per user requirement)
- [x] 2.4 Add `KillSpotify() error` method:
  - Use `syscall.Kill(-pid, syscall.SIGTERM)` (negative PID = process group)
  - Safe to call when spotifyCmd is nil (no-op)
- [x] 2.5 Modify `Close()` to call `KillSpotify()` if `spotifyCmd != nil`
- [x] 2.6 Initialize `spotifyCmd = nil` in `NewClient()`
- [x] 2.7 Add required imports: `errors`, `os`, `os/exec`, `syscall`
- [x] 2.8 Verification: `go build ./mpris/...`

## Phase 3: UI Integration (ui/shell.go)

- [x] 3.1 Add `LaunchedSpotify bool` field to `Model` struct
- [x] 3.2 Add `launchRetries int` field to `Model` struct (initialized to 0)
- [x] 3.3 Add `launchMaxRetries = 3` constant
- [x] 3.4 Modify `pollSpotifyCmd()`:
  - When `!IsRunning() && !m.LaunchedSpotify`: call `LaunchSpotify()`
  - If `LaunchSpotify` returns `ErrFlatpakSandbox`: set `m.ErrorMessage = "Spotify is running as Flatpak/Snap — D-Bus session is isolated"`, set `m.SpotifyRunning = false`
  - If `LaunchSpotify` returns any other error: increment `m.launchRetries`. If retries >= 3, set permanent error and stop polling
  - If `LaunchSpotify` succeeds: set `m.LaunchedSpotify = true`
- [x] 3.5 Modify `renderLyrics()`: apply `DefaultTheme.Error` when `m.ErrorMessage != ""`
- [x] 3.6 Modify `renderFooter()`: apply `DefaultTheme.Waiting` when `!m.SpotifyRunning`
- [x] 3.7 Replace hardcoded colors with DefaultTheme fields:
  - Header: `lipgloss.Color("15")` → `DefaultTheme.Header`
  - Footer: `lipgloss.Color("12")` → `DefaultTheme.Footer`
  - LyricActive: `lipgloss.Color("12")` → `DefaultTheme.LyricActive`
  - LyricInactive: `lipgloss.Color("15")` → `DefaultTheme.LyricInactive`
  - LyricPlain: `lipgloss.Color("7")` → `DefaultTheme.LyricPlain`
  - Visualizer: `lipgloss.Color("8")` → `DefaultTheme.Visualizer`
- [x] 3.8 Verification: `go build ./ui/...` and `go test ./ui/... -run TestShell_ResponsiveLayout`

## Phase 4: Main Entry Point (main.go)

- [x] 4.1 Before `tea.NewProgram()`: check if Spotify is running (`mpris.IsRunning()`)
- [x] 4.2 If not running, try `exec.Command("spotify").Start()` with `SysProcAttr{Setpgid: true}`
- [x] 4.3 Store the `*exec.Cmd` on the mpris client via `SetSpotifyCmd()` method
- [x] 4.4 After `p.Run()`: call `mprisClient.Close()` which calls `KillSpotify()` if `spotifyCmd != nil`
- [x] 4.5 Add signal goroutine that handles SIGINT/SIGTERM:
  - On signal, call `KillSpotify()` then `p.Quit()` and `os.Exit(0)`
- [x] 4.6 Add required imports: `os/exec`, `os/signal`, `syscall`
- [x] 4.7 Verification: `go build ./`

## Phase 5: Test Fix (ui/shell_test.go)

- [x] 5.1 Fix `TestShell_LyricHighlighting`: change position from `15 * time.Second` to `7 * time.Second`
  - 7s lies in [5s, 10s) range → active index 1 ("First verse")
  - 15s falls on "Outro" boundary → wrong index
- [x] 5.2 Verification: `go test ./ui/... -run TestShell_LyricHighlighting`

## Phase 6: Full Integration

- [x] 6.1 Run full test suite: `go test ./...`
- [x] 6.2 Verify no regressions: `go vet ./...`
- [x] 6.3 Verify formatting: `gofmt -d .`
