# Proposal: Centralized Theme & Spotify Lifecycle

## Intent

Spt-Flow's color palette is duplicated across six inline `lipgloss.Color("…")` strings in `ui/shell.go`, and its Spotify lifecycle is read-only: the TUI watches for an already-running client and shows "Waiting for Spotify…" forever otherwise. This change introduces a single `Theme` struct as the source of truth for ANSI colors, adds two new status colors (error, waiting), gives the TUI the ability to **launch** Spotify when missing and **terminate** it cleanly on exit — only when the TUI itself started it — and fixes one pre-existing failing test.

## Scope

### In Scope

- New `ui/theme.go` with a `Theme` struct (8 `lipgloss.ANSIColor` fields) and a `DefaultTheme` var carrying the current 6 colors plus Error `"9"` and Waiting `"7"`.
- Replace the 6 inline `lipgloss.Color("…")` call sites in `ui/shell.go` with `theme.Field` references.
- Add `LaunchedSpotify bool` field to `ui.Model`; on transition `!IsRunning()` inside `pollSpotifyCmd`, attempt `mpris.LaunchSpotify()` and set the flag on success.
- Add `LaunchSpotify()` and `KillSpotify()` methods on `mpris.Client`. `LaunchSpotify` uses `org.freedesktop.DBus.StartServiceByName` (`org.mpris.MediaPlayer2.spotify`, `0`). `KillSpotify` stores the `*exec.Cmd` from launch and sends `SIGTERM` to the process group.
- `main.go`: trap `SIGINT`/`SIGTERM` via `signal.Notify`, on shutdown call `mpris.KillSpotify()` if `LaunchedSpotify`.
- Run Spotify via `exec.Command("spotify")` with `SysProcAttr{Setpgid: true}` so signals stay isolated.
- Fix `TestShell_LyricHighlighting` in `ui/shell_test.go`: position `15s` is at the boundary of the last line — set the test position to `7s` (between 5s and 10s) to assert active index `1`.

### Out of Scope

- Interactive startup menu (Open Spotify / Quit / Settings) — deferred to a future cycle.
- Re-launching Spotify after the user closes it mid-session (one-shot launch only).
- Theme switching / persistence (no config file, no env overrides beyond `FLATPAK_ID` warning).
- Refactoring `mpris.Client` beyond the two new methods.

## Capabilities

### New Capabilities

- `ui-theme`: centralized color palette and `Theme` struct.
- `spotify-lifecycle`: launch-on-missing and kill-on-exit subprocess management for Spotify.

### Modified Capabilities

- None. This change adds new behavior without altering existing spec-level requirements (the "Waiting for Spotify…" fallback stays as a fallback; D-Bus read contract unchanged).

## Approach

**Theme**: introduce `ui/theme.go` exposing `var DefaultTheme = Theme{Header: "15", Footer: "12", LyricActive: "12", LyricInactive: "15", LyricPlain: "7", Visualizer: "8", Error: "9", Waiting: "7"}`. Each existing call site reads `DefaultTheme.Header` etc. No model change. Two unused colors (Error, Waiting) become available for the "Waiting for Spotify…" screen and any future error/wait banners.

**Launch**: in `pollSpotifyCmd`, when `IsRunning()` returns false, attempt `LaunchSpotify()`. On success set `m.LaunchedSpotify = true`. On failure, leave flag false and surface the error through `SpotifyStateMsg.Err`. Existing 2s `PollTickMsg` covers the 3–8s Spotify startup latency.

**Kill**: `mpris.Client` stores `cmd *exec.Cmd`. `main.go` installs a `signal.Notify` channel, runs `tea.NewProgram`, and on signal calls `client.Kill()` then exits. `KillSpotify` sends `SIGTERM` to `cmd.Process.Pid`; `Setpgid` prevents signal bleed to the TUI process.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `ui/theme.go` | New | Theme struct + `DefaultTheme` |
| `ui/shell.go` | Modified | 6 color call sites use `DefaultTheme`; `LaunchedSpotify` field added; `pollSpotifyCmd` calls `LaunchSpotify` |
| `mpris/client.go` | Modified | `LaunchSpotify`, `KillSpotify`, `cmd *exec.Cmd` |
| `main.go` | Modified | `signal.Notify` trap, kill on shutdown |
| `ui/shell_test.go` | Modified | Fix lyric-position test value |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Flatpak/Snap Spotify sandbox isolates session D-Bus | Med | Detect `FLATPAK_ID` env var; emit warning to stderr before attempting launch |
| Spotify takes 3–8s to register on D-Bus after launch | Med | Existing 2s `PollTickMsg` already retries `IsRunning()`; flag stays true on first success |
| `SIGTERM` propagated to TUI process via shared pgid | Med | `SysProcAttr{Setpgid: true}` on Spotify subprocess isolates the process group |
| Killing Spotify the user already had open | Low | `LaunchedSpotify` flag set only after `LaunchSpotify` success; `KillSpotify` no-ops when flag false |
| Flatpak Spotify launches via `flatpak run` not `spotify` | Low | If `exec.LookPath("spotify")` fails, surface actionable error message |

## Rollback Plan

Revert the single commit (or PR slice). No schema, no migration. The `ui/theme.go` file and two new `mpris` methods can be removed independently; `Model.LaunchedSpotify` defaults to `false`, restoring pre-change behavior. Failing test pre-existed — revert restores the original (broken) assertion.

## Dependencies

- `github.com/godbus/dbus/v5` — already in `go.mod`; uses existing `org.freedesktop.DBus.StartServiceByName`.
- `os/exec`, `os/signal`, `syscall` — Go stdlib.
- Spotify binary on `$PATH` (`/usr/bin/spotify` or Flatpak wrapper). Not a project dependency — runtime prerequisite only.

## Success Criteria

- [ ] All 6 inline `lipgloss.Color("…")` literals in `ui/shell.go` replaced by `DefaultTheme.*` references.
- [ ] `ui/theme.go` exports `Theme` and `DefaultTheme` with 8 fields.
- [ ] When Spotify is not running, TUI attempts `LaunchSpotify()` once and tracks the result in `Model.LaunchedSpotify`.
- [ ] On SIGINT/SIGTERM, TUI sends SIGTERM only to the Spotify process group it launched; user-pre-existing Spotify is never killed.
- [ ] `go test ./...` passes including `TestShell_LyricHighlighting`.
- [ ] Flatpak launch path emits a clear stderr warning instead of silently failing.

## File Inventory

- **New**: `ui/theme.go`
- **Modified**: `ui/shell.go`, `ui/shell_test.go`, `mpris/client.go`, `main.go`
- **No change**: `lyrics/*`, `ui/visualizer.go`, `ui/visualizer_test.go`, `go.mod`, `README.md`

## Dependency Graph (apply order)

```
ui/theme.go                  (leaf — no deps)
    └─► ui/shell.go          (depends on theme)
            └─► ui/shell_test.go   (depends on shell + theme)
mpris/client.go              (adds 2 methods, no new imports blocking)
    └─► ui/shell.go          (calls LaunchSpotify)
            └─► main.go      (signal trap + KillSpotify call)
```

Apply order: theme → shell (colors + LaunchedSpotify + launch call) → mpris (LaunchSpotify + KillSpotify) → main (signal trap) → tests.

## Open Questions

1. Should `Error` color also be wired into `renderLyrics` when `m.ErrorMessage != ""`? (Proposal keeps that wiring for a follow-up to avoid expanding scope.)
2. Should the Flatpak warning be a fatal error or a non-blocking notice? Proposal assumes non-blocking; user can choose.
3. If `LaunchSpotify` returns an error, should the TUI keep retrying every poll or give up after N attempts? Proposal keeps infinite retry (matches current "Waiting…" behavior).