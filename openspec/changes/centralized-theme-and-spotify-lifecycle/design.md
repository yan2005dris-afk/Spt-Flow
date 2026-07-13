# Design: Centralized Theme & Spotify Lifecycle

## Technical Approach

Two capabilities, one change: centralize the TUI's color palette (`ui-theme`) and own Spotify's lifecycle from pre-launch to clean shutdown (`spotify-lifecycle`). Four files modified, one new file, tight coupling between `ui` and `mpris` for the new state machine.

**Architecture pattern: Process-Group Isolation + Capability Flags.** The `mpris.Client` owns the Spotify process handle via `spotifyCmd *exec.Cmd`. The `ui.Model` owns user-visible launch state via `LaunchedSpotify bool`. A package-level `Theme` var (NOT a Model field, per spec) owns the color palette. Signals flow OS → `main.go` → `mpris client` → `SIGTERM` to the Spotify process group only.

## Architecture Decisions

### Decision: Launch-via-D-Bus primary, `exec.Command("spotify")` fallback in main.go

| Option | Tradeoff | Decision |
|--------|----------|----------|
| D-Bus `StartServiceByName` only | Cleanest, fails when no `.desktop` activation handler is registered | ❌ |
| `exec.Command("spotify")` only | Always works, but exposes implementation detail in caller | ❌ |
| **D-Bus retry path in `mpris` + `exec.Command` pre-launch in `main.go`** | Covers both activation paths; process-group isolation prevents SIGTERM bleed-back | ✅ |

**Rationale**: D-Bus activation is the OS-blessed way to start a service when no well-known name is owned; `exec.Command` is the universal fallback when no D-Bus activation handler exists (uncommon distros, manual Spotify installs). `Setpgid: true` on the fallback subprocess is critical — without it, killing Spotify also kills the TUI because both share the parent's process group by default.

### Decision: `spotifyCmd` lives on `mpris.Client`, not on `ui.Model`

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `ui.Model.spotifyCmd` | Matches "View owns runtime state" idiom in Bubble Tea | ❌ |
| **`mpris.Client.spotifyCmd` field** | Co-locates the OS handle with the D-Bus handle in one resource owner | ✅ |
| Standalone package-level handle | Forces global state | ❌ |

**Rationale**: `mpris.Client` already owns `realConn *dbus.Conn` for the D-Bus connection lifetime. The exec handle has the same lifecycle and ownership semantics. `ui.Model` only needs the user-visible flag `LaunchedSpotify`, which has different lifecycle semantics (a UI hint, not a resource handle).

### Decision: `Theme` is package-level, not on `Model`

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `Model.Theme` field | Allows per-instance theming at runtime | ❌ (over-engineered, no current need) |
| **Package-level `DefaultTheme` var** | Single source of truth, no Model bloat, trivial access in render funcs | ✅ (per spec) |

**Rationale**: Spec requirement RQ-5 mandates no Theme on Model. No current need for multi-instance theming; trivial to migrate later if needed.

### Decision: Flatpak detected → `ErrFlatpakSandbox` permanent error, halt polling

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Silent warning, keep polling | Aligns with the proposal's "non-blocking notice" alternative | ❌ |
| **Permanent error, halt polling** | User cannot recover without uninstalling the sandboxed Spotify | ✅ (per spec scenario) |

**Rationale**: Flatpak/Snap Spotify runs in a session-bus sandbox that is structurally incompatible with this TUI's D-Bus model. Continuing to poll wastes cycles and confuses users with an apparently-alive daemon they cannot control.

### Decision: Process kill via negative-PID `syscall.Kill` (process-group semantics)

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `cmd.Process.Kill()` (positive PID, just this process) | Kills Spotify main; child helper processes (Spotify Helper, Web Helper) survive orphaned | ❌ |
| **`syscall.Kill(-pid, SIGTERM)` (negative = process group)** | Single signal cleans up the entire Spotify process tree | ✅ |

**Rationale**: Spotify spawns multiple helper processes. `Setpgid: true` puts them in their own pgid so a single signal reaches all of them.

## Data Flow

```
main.main()
   ├── mpris.NewClient()                 ──► spotifyCmd = nil
   ├── if !IsRunning():                   ──► exec.Command("spotify")
   │       SysProcAttr{Setpgid: true}        Setpgid breaks signal pipe
   │       mprisClient.spotifyCmd = cmd     resource handle stays on Client
   │       cmd.Start()
   ├── tea.NewProgram(m).Run()
   │     │
   │     └── pollSpotifyCmd (every 2s via PollTickMsg)
   │           if !IsRunning() && !LaunchedSpotify:
   │              LaunchSpotify()             ──► D-Bus StartServiceByName
   │              on ErrFlatpakSandbox:       ──► ErrorMessage = "..."; STOP
   │              on generic error:           ──► launchRetries++; max 3
   │              on success:                 ──► LaunchedSpotify = true
   │
   └── SIGINT/SIGTERM                      ──► signal goroutine
            if mprisClient.spotifyCmd != nil:
                syscall.Kill(-pid, SIGTERM)  ──► entire Spotify pgid dies
            p.Quit()                        ──► TUI exits cleanly
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `ui/theme.go` | Create | `Theme` struct (8 fields) + `DefaultTheme` package var |
| `ui/shell.go` | Modify | Replace 6 `lipgloss.Color("…")` sites with `DefaultTheme.*`; add `LaunchedSpotify bool`, `launchRetries int`, `launchMaxRetries = 3`; extend `pollSpotifyCmd()` |
| `ui/shell_test.go` | Modify | Fix `TestShell_LyricHighlighting`: position `15s` → `7s` (active index 1) |
| `mpris/client.go` | Modify | Add `ErrFlatpakSandbox`, `spotifyCmd *exec.Cmd`; add `LaunchSpotify()`, `KillSpotify()`; `NewClient()` inits `spotifyCmd = nil`; `Close()` calls `KillSpotify()` first |
| `main.go` | Modify | Pre-launch path (`exec.Command("spotify")` + `Setpgid`); `signal.Notify` goroutine for `SIGINT/SIGTERM`; on signal → `KillSpotify()` → `p.Quit()` |

### `ui/theme.go` (NEW FILE)

```go
package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
    Header        lipgloss.ANSIColor
    Footer        lipgloss.ANSIColor
    LyricActive   lipgloss.ANSIColor
    LyricInactive lipgloss.ANSIColor
    LyricPlain    lipgloss.ANSIColor
    Visualizer    lipgloss.ANSIColor
    Error         lipgloss.ANSIColor
    Waiting       lipgloss.ANSIColor
}

var DefaultTheme = Theme{
    Header:        lipgloss.Color("15"),
    Footer:        lipgloss.Color("12"),
    LyricActive:   lipgloss.Color("12"),
    LyricInactive: lipgloss.Color("15"),
    LyricPlain:    lipgloss.Color("7"),
    Visualizer:    lipgloss.Color("8"),
    Error:         lipgloss.Color("9"),
    Waiting:       lipgloss.Color("7"),
}
```

### `mpris/client.go` CHANGES — key sections

```go
// New imports:
//   "errors"
//   "os"
//   "os/exec"
//   "syscall"

var ErrFlatpakSandbox = errors.New(
    "spotify runs in flatpak/snap sandbox; not compatible with this TUI",
)

type Client struct {
    conn       DBusConnection
    obj        DBusObject
    realConn   *dbus.Conn
    spotifyCmd *exec.Cmd // nil when TUI did not launch the process
}

func NewClient() (*Client, error) {
    conn, err := dbus.ConnectSessionBus()
    if err != nil {
        return nil, err
    }
    adapterConn := &realDBusConnection{conn: conn}
    obj := adapterConn.Object("org.mpris.MediaPlayer2.spotify", "/org/mpris/MediaPlayer2")
    return &Client{
        conn:       adapterConn,
        obj:        obj,
        realConn:   conn,
        spotifyCmd: nil,
    }, nil
}

func (c *Client) Close() error {
    // Always clean up the subprocess handle first so we don't leak a
    // Spotify that the TUI started but couldn't kill on exit.
    if c.spotifyCmd != nil {
        _ = c.KillSpotify()
    }
    if c.realConn != nil {
        return c.realConn.Close()
    }
    return nil
}

// LaunchSpotify asks D-Bus to activate the Spotify MPRIS service. Returns
// ErrFlatpakSandbox when the running Spotify is sandboxed and unreachable
// from the user's session bus; the caller MUST stop polling on this error.
func (c *Client) LaunchSpotify() error {
    if os.Getenv("FLATPAK_ID") != "" && !c.IsRunning() {
        return ErrFlatpakSandbox
    }
    call := c.conn.Call(
        "org.freedesktop.DBus.StartServiceByName",
        0,
        "org.mpris.MediaPlayer2.spotify",
        uint32(0),
    )
    return call.Err
}

// KillSpotify sends SIGTERM to the TUI-launched Spotify process group.
// Negative PID addresses the whole pgid (requires Setpgid: true on Cmd).
// Safe to call when spotifyCmd is nil — no-op.
func (c *Client) KillSpotify() error {
    if c.spotifyCmd == nil || c.spotifyCmd.Process == nil {
        return nil
    }
    return syscall.Kill(-c.spotifyCmd.Process.Pid, syscall.SIGTERM)
}
```

### `ui/shell.go` CHANGES — key sections

```go
const launchMaxRetries = 3

type Model struct {
    // ... all existing fields ...
    LaunchedSpotify bool // set true after first successful LaunchSpotify
    launchRetries   int  // consecutive failed launches; capped at launchMaxRetries
}

func (m *Model) pollSpotifyCmd() tea.Cmd {
    return func() tea.Msg {
        if m.MprisClient == nil {
            client, err := mpris.NewClient()
            if err != nil {
                return SpotifyStateMsg{Running: false, Err: err}
            }
            m.MprisClient = client
        }

        if !m.MprisClient.IsRunning() {
            // Attempt to launch Spotify the FIRST time (and on retries).
            if !m.LaunchedSpotify && m.launchRetries < launchMaxRetries {
                if err := m.MprisClient.LaunchSpotify(); err != nil {
                    if errors.Is(err, mpris.ErrFlatpakSandbox) {
                        m.ErrorMessage =
                            "Spotify is running as Flatpak/Snap — not compatible with this TUI"
                        m.SpotifyRunning = false
                        return SpotifyStateMsg{Running: false, Err: err}
                    }
                    m.launchRetries++
                    if m.launchRetries >= launchMaxRetries {
                        m.ErrorMessage = "Failed to launch Spotify after 3 attempts"
                        m.SpotifyRunning = false
                        return SpotifyStateMsg{Running: false, Err: err}
                    }
                    return SpotifyStateMsg{Running: false, Err: err}
                }
                m.LaunchedSpotify = true
            }
            return SpotifyStateMsg{Running: false}
        }

        // ... existing IsRunning==true body (GetTrack/GetPlaybackStatus/...) ...
    }
}
```

**Color replacements** (six total — see spec for the table):

```go
// renderHeader  : lipgloss.Color("15") → DefaultTheme.Header
// renderFooter  : lipgloss.Color("12") → DefaultTheme.Footer
// renderLyrics  : lipgloss.Color("12") active  → DefaultTheme.LyricActive
// renderLyrics  : lipgloss.Color("15") inactive → DefaultTheme.LyricInactive
// renderLyrics  : lipgloss.Color("7")  plain    → DefaultTheme.LyricPlain
// renderVisualizer: lipgloss.Color("8")        → DefaultTheme.Visualizer
```

New conditional coloring in render funcs:

```go
// renderLyrics: when error message is set, render the message in DefaultTheme.Error
if m.ErrorMessage != "" {
    return DefaultTheme.Error.NewStyle().
        Width(width).
        Align(lipgloss.Center).
        Render(m.ErrorMessage)
}

// renderFooter: when Spotify is not running, render "Waiting…" footer in DefaultTheme.Waiting
```

### `main.go` CHANGES

```go
package main

import (
    "fmt"
    "os"
    "os/exec"
    "os/signal"
    "syscall"

    "tui-spotify/mpris"
    "tui-spotify/ui"

    tea "github.com/charmbracelet/bubbletea"
)

func main() {
    mprisClient, err := mpris.NewClient()
    if err != nil {
        fmt.Fprintf(os.Stderr, "mpris: %v\n", err)
        os.Exit(1)
    }

    // Pre-launch Spotify ONLY when it isn't already running. We hold the
    // exec.Cmd handle on the Client so KillSpotify can use it later.
    // SysProcAttr{Setpgid: true} is mandatory: it isolates the Spotify
    // process group so SIGTERM does not propagate back into the TUI.
    if !mprisClient.IsRunning() {
        cmd := exec.Command("spotify")
        cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
        if startErr := cmd.Start(); startErr == nil {
            mprisClient.spotifyCmd = cmd
        }
        // cmd.Start() failure is non-fatal: LaunchSpotify will retry via D-Bus.
    }

    p := tea.NewProgram(ui.NewModel(), tea.WithAltScreen())

    // Signal trap: catch Ctrl+C and SIGTERM before the bubble tea loop
    // swallows them. defer-style cleanup is enforced by KillSpotify's
    // nil-guard inside Client.Close().
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigCh
        if mprisClient != nil && mprisClient.spotifyCmd != nil {
            _ = mprisClient.KillSpotify()
        }
        p.Quit()
    }()

    if _, err := p.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
        os.Exit(1)
    }

    // Final safety net: even when SIGINT/SIGTERM did NOT fire (e.g. normal
    // exit via 'q' key), kill Spotify if we launched it. Client.Close is
    // idempotent thanks to the nil-guard on spotifyCmd.
    _ = mprisClient.Close()
}
```

### `ui/shell_test.go` FIX

```diff
-        Position: 15 * time.Second, // between 5s and 10s -> "First verse" is active
+        Position: 7 * time.Second,  // 7s lies in [5s, 10s)  -> active index 1 ("First verse")
```

The 15s position falls EXACTLY on the "Outro" timestamp (15s), so `getActiveLyricIndex` returns 3, not 1. The 7s position falls inside the [5s, 10s) range of line index 1 ("First verse"), satisfying the original assertion.

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit (`ui`) | Theme field count and color values, lyric index math, conditional color in `renderLyrics`/`renderFooter` | Extend `ui/shell_test.go`: table-driven test for `getActiveLyricIndex`; visual snapshot test for Theme integration |
| Unit (`mpris`) | `LaunchSpotify` issues D-Bus call with correct args, `KillSpotify` no-op on nil cmd, Flatpak detection returns `ErrFlatpakSandbox` | Extend `mpris/client_test.go`: mock `DBusConnection`, inject `os.Setenv("FLATPAK_ID", "")` and test both branches |
| Integration | Pre-launch flow, signal-based cleanup, polling state machine | Manual E2E: stop Spotify, run `tui-spotify`, observe pre-launch; SIGINT to running instance, verify Spotify dies and TUI exits cleanly |
| Regression | `TestShell_LyricHighlighting` 15s→7s assertion passes | `go test ./ui/... -run TestShell_LyricHighlighting` |

## Threat Matrix

**Applicable** — this change touches subprocess management (`exec.Command`), signal handling (`signal.Notify`, `syscall.Kill`), and D-Bus activation. Rows marked N/A have explicit reasons.

| Boundary | Cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| **Subprocess launch** | `exec.Command("spotify")` with `Setpgid: true`; `exec.LookPath` failure; `FLATPAK_ID` env; `cmd.Start()` error | **Applicable** | `Setpgid` always set; `cmd.Start()` failure logged but NOT fatal — polling retries via D-Bus; `FLATPAK_ID` checked before any launch attempt | RED: `TestMprisClient_PreLaunch_SetsSetpgid`; RED: `TestMprisClient_FlatpakReturnsErr`; RED: `TestMain_DBusFallbackOnExecFailure` |
| **Process signaling** | `SIGTERM` to process group via negative PID; signal goroutine isolation; signal-channel buffer | **Applicable** | `syscall.Kill(-pid, SIGTERM)` for whole-pgid cleanup; `signal.Notify` in goroutine (NOT main thread); buffered channel size 1 prevents signal loss | RED: `TestMprisClient_KillSpotify_NoOpOnNilCmd`; RED: `TestMprisClient_KillSpotify_NegativePID`; manual E2E |
| **Environment detection** | `FLATPAK_ID`; `SNAP_NAME`; unset env | **Applicable** | Check `FLATPAK_ID` only (per spec) when `IsRunning()` is false; add `SNAP_NAME` as future enhancement | RED: `TestMprisClient_LaunchSpotify_FlatpakEnv` returns `ErrFlatpakSandbox`; RED: empty env returns nil |
| **Polling state machine** | First poll (no Spotify), retry counter, permanent error after `launchMaxRetries`, Flatpak terminal state | **Applicable** | `m.LaunchedSpotify` set only after success; `m.launchRetries` increments on generic error only; Flatpak halts polling by setting `m.SpotifyRunning = false` and storing error | RED: `TestShell_LaunchSpotify_RetryCounter`; RED: `TestShell_LaunchSpotify_FlatpakHaltsPolling`; RED: `TestShell_LaunchSpotify_MaxRetriesSetsError` |
| Documentation-like paths | N/A — no executable Markdown or README scripts in this change | N/A: not a docs-tooling change | — | — |
| Git/PR automation | N/A — no `git -C` or `gh` integration touched | N/A: orchestrator responsibility, not in this change | — | — |

## Migration / Rollout

No schema migration, no data migration, no feature flag. Rollout steps:

1. Merge the five files in dependency order (see Dependency Order section).
2. Each step is independently testable (see Verification section).
3. Rollback = revert the single commit. `Model.LaunchedSpotify` defaults to `false`, restoring pre-change behavior. Pre-existing failing test (`TestShell_LyricHighlighting`) is fixed in the same commit (no separate hotfix needed).

## Risk Mitigations

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| **SIGTERM bleed-back to TUI** — both share parent's pgid by default | Med | `SysProcAttr{Setpgid: true}` + negative-PID `syscall.Kill` sends SIGTERM to Spotify pgid only |
| **Killing user-pre-existing Spotify** | Low | `spotifyCmd` field set ONLY when main.go pre-launch succeeds; `KillSpotify` no-ops on nil cmd; `IsRunning()` check before pre-launch |
| **Flatpak/Snap session-bus isolation** — Spotify unreachable from user session bus | Med | `FLATPAK_ID` env checked in `LaunchSpotify`; returns `ErrFlatpakSandbox` (terminal, not retryable) |
| **3–8s D-Bus registration latency after pre-launch** | High | Pre-`exec.Command` starts Spotify BEFORE the TUI's first poll; existing 2s `PollTickMsg` retries `IsRunning()` until registered |
| **D-Bus-mediated launch leaves no `*exec.Cmd` handle** (Spotify started by D-Bus activation, not by us) | Med | If user closes Spotify externally, `KillSpotify` no-ops via nil guard; acceptable trade-off — user is responsible |
| **Test `TestShell_LyricHighlighting` regression** — 15s is on the "Outro" boundary | Med | Position changed to `7s` (in [5s, 10s) window); comment in test explains why |
| **Race between `cmd.Start()` and D-Bus registration** — first poll sees `IsRunning() == false` and tries D-Bus despite us just having launched it | Low | Pre-launch is idempotent with D-Bus retry: LaunchedSpotify flag prevents double-launch via D-Bus path |
| **`signal.Notify` goroutine exits without cleanup** if TUI panics | Low | Two layers of safety: signal goroutine calls `p.Quit()`; main.go also calls `mprisClient.Close()` after `p.Run()` returns, which itself guards via nil check |
| **`exec.Command("spotify")` not on PATH** | Med | `cmd.Start()` returns error; logged to stderr; code path falls through to D-Bus retry in `pollSpotifyCmd` |

## How to Verify Each Piece Independently

| Step | Verification |
|------|--------------|
| **1. `ui/theme.go` only** | `go build ./ui/...` succeeds; `go test ./ui/... -run TestShell_ResponsiveLayout` passes (no color-literal regressions) |
| **2. `mpris/client.go` only** | `go test ./mpris/...` passes; existing `TestMprisClient_IsRunning` still validates the mock connection; new tests added for `LaunchSpotify`/`KillSpotify` cover the new paths |
| **3. `ui/shell.go` color replacement** | `go test ./ui/... -run TestShell_ResponsiveLayout` and `-run TestShell_SpotifyOfflineFallback` pass (no rendering regressions); existing rendering is byte-identical because `DefaultTheme` colors match the previous hardcoded ones |
| **4. `ui/shell.go` launch logic** | New unit tests `TestShell_LaunchSpotify_RetryCounter` and `TestShell_LaunchSpotify_FlatpakHaltsPolling` pass |
| **5. `main.go` only** | `go build ./` succeeds; manual: stop Spotify → run `tui-spotify` → `ps aux \| grep spotify` shows new process with new pgid |
| **6. `ui/shell_test.go` fix** | `go test ./ui/... -run TestShell_LyricHighlighting` passes |
| **7. Full suite** | `go test ./...` → 0 failures; linter `gofmt -d .` clean |

## Dependency Order (apply order)

```
ui/theme.go                    (leaf, no deps)
   └─► ui/shell.go             (uses DefaultTheme + mpris.ErrFlatpakSandbox)
         └─► ui/shell_test.go  (depends on shell + theme)

mpris/client.go                (adds 2 methods, depends on dbus + stdlib exec/syscall)
   └─► ui/shell.go             (calls LaunchSpotify, KillSpotify, accesses ErrFlatpakSandbox)
         └─► main.go           (depends on mpris + ui)
```

**Apply order**:

1. **`ui/theme.go`** — leaf, unblocks color refactor.
2. **`mpris/client.go`** — unblocks the launch logic in `ui/shell.go`.
3. **`ui/shell.go`** — consumes both `theme` and `mpris` additions.
4. **`main.go`** — last because it composes all the pieces (mpris client + ui model + signal trap).
5. **`ui/shell_test.go`** — fixed last after all production changes so the regression is observed in context.

Each step compiles and tests independently; only the final step yields the full user-visible behaviour.

## Open Questions

1. **`LaunchSpotify` exec fallback ambiguity** — User's section 1 says "D-Bus only, no exec.Command fallback" while section 4 places an `exec.Command("spotify")` in `main.go`. The design above treats them as **two distinct paths** (exec happens eagerly in main.go; D-Bus only happens lazily in `LaunchSpotify()`). The apply phase MUST confirm: should `LaunchSpotify()` internally fall back to exec when D-Bus activation returns "no handler", OR should `LaunchSpotify()` stay strictly D-Bus-only and rely on main.go's pre-launch for the exec path?
2. **Should we also detect `SNAP_NAME`?** — The spec only requires `FLATPAK_ID`. Adding `SNAP_NAME` is a one-liner extension; decide before apply.
3. **Linux-only `Pdeathsig`** — Optional: `cmd.SysProcAttr.Pdeathsig = syscall.SIGTERM` makes Spotify auto-terminate if the TUI dies abnormally (e.g. panic, kill -9). Costs a Linux-only import. Decide based on portability goals.
4. **`Defer` cleanup in main** — Should `mprisClient.Close()` be wrapped in `defer mprisClient.Close()` immediately after construction, or kept as the post-`p.Run()` safety net? Idempotent in either case; minor stylistic preference.
