# Design: librespot-backend

## Technical Approach

Replace `Spotify desktop` as the default backend with `librespot` (headless Spotify Connect daemon), launched from the menu via `exec.Command` instead of D-Bus `StartServiceByName`. Spot-check conflict with Spotify desktop before launching via D-Bus `GetNameOwner`. Add `--setup` one-time OAuth flow to `main.go`. Keep `LaunchSpotify()` as the secondary path for menu option 5 (now `"Start with Spotify Desktop"`). `pollSpotifyCmd()` becomes a pure observer — no auto-launch.

This implementation satisfies the `librespot-backend` spec (new `LaunchLibrespot` / `IsSpotifyDesktopRunning` / error types / `--setup`) and the `spotify-lifecycle` delta (D-Bus is now secondary; menu owns launching).

## Architecture Decisions

| Decision | Choice | Alternatives considered | Rationale |
|---|---|---|---|
| Launch mechanism | `exec.Command("librespot", ...)` from menu | D-Bus `StartServiceByName` (librespot registers a name too) | `exec.Command` gives us a `*exec.Cmd` handle → `KillSpotify` works via process group. D-Bus-launched librespot would have no parent to kill cleanly. |
| OAuth mode | `--enable-oauth` with stdout scanner for URL + "Authenticated as" marker | Device-code auth, env-var token pre-seed | librespot stores tokens in `--cache`; one-time flow is enough. `--enable-oauth` is the canonical headless path. |
| Conflict detection | `IsSpotifyDesktopRunning()` via `GetNameOwner("org.mpris.MediaPlayer2.spotify")` | Try-launch-and-rollback; D-Bus NameHasOwner | D-Bus name ownership is the only authoritative signal; both Spotify desktop and librespot can claim the standard name but only Spotify desktop has it before our launch. Returning the owner distinguishes them when called BEFORE launch, and `GetNameOwner` errors when nobody owns it (== safe to launch librespot). |
| `spotifyCmd` ownership | `*mpris.Client` holds it; menu pushes via `SetSpotifyCmd` | Per-launch local var in main.go | Existing pattern; `Close()` already guards on nil. |
| Menu option ordering | 5 options, Help stays at 4, Spotify Desktop added at 5 (per design prompt) | Help renumbers to 6 (per `specs/librespot-backend/spec.md` line 60) | **Conflict with spec.** Following user design prompt; flagging as Open Question #1. |
| `LaunchSpotify()` retention | Keep, used by option 5 | Remove | Backward-compatible for explicit `IsRunning() == false` paths and edge cases. |
| `--setup` UX | Stream stdout, highlight URL, wait for "Authenticated as" / "Connected to AP" | Open browser via xdg-open, single-shot run until MPRIS Ready | Stream-and-detect matches librespot's log style. User explicitly suggests this approach. |

## Data Flow

### Option 1 — Launch librespot

```
User selects "Start with Librespot + TUI"
        |
        v
ui.MenuModel.Update --> MenuChoiceMsg{Choice: ChoiceStartLibrespot}
        |
        v
ui.shell.go: case ChoiceStartLibrespot
        |
        +--> mpris.NewClient()
        +--> mpris.IsSpotifyDesktopRunning()?
        |       |-- true  --> m.ErrorMessage = "Spotify desktop is running..."
        |       \-- false --> mpris.LaunchLibrespot(cfg)
        |                       |-- ErrLibrespotNotInstalled --> install banner
        |                       \-- ok                     --> SetSpotifyCmd + LaunchedSpotify=true
        v
ViewState = "tui" --> pollSpotifyCmd() observes IsRunning()=true
```

### `--setup` flow

```
main.go: --setup flag
        |
        v
runSetup()
   1. exec.Lookup("librespot")? err --> print install instructions, exit 1
   2. cmd = exec.Command("librespot", --name, --enable-oauth, --cache, --config, ...)
      cmd.StdoutPipe() --> bufio.Scanner
      cmd.Stderr = os.Stderr
   3. cmd.Start(); defer cmd.Process.Wait()
   4. for each line:
        - print line
        - if contains "https://" --> print "^ Open this URL in your browser"
        - if contains "Authenticated as" || "Connected to AP" --> print success, break
   5. exit 0
```

### Kill on exit

```
main.go signal handler (SIGINT/SIGTERM) OR main.go defer mprisClient.Close()
        |
        v
mpris.KillSpotify(): syscall.Kill(-spotifyCmd.Process.Pid, SIGTERM)
        ^--- Setpgid:true (set in LaunchLibrespot) ensures pid == pgid;
             negative PID targets the whole group, ONLY the librespot subtree.
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `mpris/client.go` | Modify | Add `ErrLibrespotNotInstalled`, `ErrSpotifyDesktopRunning`, `IsSpotifyDesktopRunning()`, `LaunchLibrespot()`. Keep `LaunchSpotify()` and `KillSpotify()` unchanged. |
| `mpris/client_test.go` | Modify | Add tests for `IsSpotifyDesktopRunning`, `LaunchLibrespot` (binary-not-found, desktop-running, success-mock). |
| `ui/menu.go` | Modify | Add `ChoiceStartLibrespot` and `ChoiceStartSpotifyDesktop` constants. Update `MenuModel.Update` to wrap modulo 5. Update `View()` and `renderMenuView()` to render 5 options. |
| `ui/shell.go` | Modify | Add `ChoiceStartLibrespot` case (uses `IsSpotifyDesktopRunning()` + `LaunchLibrespot()`). Add `ChoiceStartSpotifyDesktop` case (existing spotify-exec.Command logic). Update `pollSpotifyCmd` comment to document no-auto-launch (logic already correct: `if !IsRunning() return SpotifyStateMsg{Running:false}`). Update `SelectedMenuOption` modulo from 4 to 5. |
| `ui/shell_test.go` | Modify | Update `TestMenu_Render` for new labels; update `TestMenu_Navigate` modulo 5; add `TestMenu_SelectStartLibrespot` and `TestMenu_SelectStartSpotifyDesktop`. |
| `main.go` | Modify | Pre-loop argv scan for `--setup` and `--help`. Add `runSetup()`, `printInstallInstructions()`, `printUsage()`. Reuse `mpris.ConfigPath()` helper. |

No deletions. `LaunchSpotify()` stays for backward compat.

## Interfaces / Contracts

### New error sentinels (`mpris/client.go`)

```go
var ErrLibrespotNotInstalled = errors.New("librespot not found in PATH")
var ErrSpotifyDesktopRunning = errors.New(
    "Spotify desktop is running - please close it first",
)
```

### New client methods

```go
// IsSpotifyDesktopRunning reports whether org.mpris.MediaPlayer2.spotify
// is owned. librespot also claims this name when running, so this is only
// safe to call BEFORE launching librespot. The contract is: at the moment
// of the call, if the name has an owner, it is Spotify desktop (librespot
// is not yet launched in this session).
func (c *Client) IsSpotifyDesktopRunning() bool

// LaunchLibrespot starts the librespot binary with OAuth and returns the
// cmd so the caller can store it via SetSpotifyCmd. cfg may be empty for
// the user-default ~/.config/librespot.conf. Returns ErrLibrespotNotInstalled
// or ErrSpotifyDesktopRunning on the documented conditions.
func (c *Client) LaunchLibrespot(cfg string) error
```

### New menu constants (`ui/menu.go`)

```go
const (
    ChoiceStartLibrespot      = "start-librespot"       // new
    ChoiceStartSpotifyDesktop = "start-spotify-desktop" // new
    ChoiceTUIOnly             = "tui-only"
    ChoiceCheckStatus         = "check-status"
    ChoiceHelp                = "help"
)
```

### Internal contract — `runSetup` signature

```go
// runSetup executes the OAuth one-time setup. Exits via os.Exit(1) on
// missing binary (so main does not proceed into the TUI).
func runSetup() error
```

## Code Snippets (load-bearing)

### `mpris/client.go` — `LaunchLibrespot`

```go
func (c *Client) LaunchLibrespot(cfg string) error {
    path, err := exec.LookPath("librespot")
    if err != nil {
        return ErrLibrespotNotInstalled
    }
    if c.IsSpotifyDesktopRunning() {
        return ErrSpotifyDesktopRunning
    }

    cache, _ := os.UserHomeDir() // ignored error: same fallback as spec
    args := []string{
        "--name", "Spt-Flow",
        "--enable-oauth",
        "--cache", filepath.Join(cache, ".cache/librespot"),
    }
    if cfg != "" {
        args = append(args, "--config", cfg)
    }

    cmd := exec.Command(path, args...)
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    if err := cmd.Start(); err != nil {
        return fmt.Errorf("failed to start librespot: %w", err)
    }
    c.SetSpotifyCmd(cmd)
    return nil
}
```

### `main.go` — `runSetup`

```go
func runSetup() error {
    path, err := exec.LookPath("librespot")
    if err != nil {
        printInstallInstructions()
        return ErrLibrespotNotInstalled
    }
    cfg := filepath.Join(mustHome(), ".config/tui-spotify/librespot.conf")
    cmd := exec.Command(path,
        "--name", "Spt-Flow",
        "--enable-oauth",
        "--cache", filepath.Join(mustHome(), ".cache/librespot"),
        "--config", cfg,
    )
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return err
    }
    cmd.Stderr = os.Stderr
    if err := cmd.Start(); err != nil {
        return fmt.Errorf("failed to start librespot: %w", err)
    }
    defer func() { _ = cmd.Process.Wait() }()

    scanner := bufio.NewScanner(stdout)
    scanner.Buffer(make([]byte, 64*1024), 1024*1024)
    for scanner.Scan() {
        line := scanner.Text()
        fmt.Println(line)
        if strings.Contains(line, "https://") {
            fmt.Println("^ Open this URL in your browser to authorize")
        }
        if strings.Contains(line, "Authenticated as") ||
            strings.Contains(line, "Connected to AP") {
            fmt.Println("Authentication successful!")
            return nil
        }
    }
    return fmt.Errorf("librespot exited before authentication")
}

func mustHome() string {
    h, _ := os.UserHomeDir()
    return h
}
```

### `ui/shell.go` — `MenuChoiceMsg` handler

```go
case MenuChoiceMsg:
    switch msg.Choice {
    case ChoiceStartLibrespot:
        client, err := mpris.NewClient()
        if err != nil {
            m.ErrorMessage = err.Error()
            return m, menuTickCmd(3 * time.Second)
        }
        m.MprisClient = client
        cfg := filepath.Join(mustHome(), ".config/tui-spotify/librespot.conf")
        if err := client.LaunchLibrespot(cfg); err != nil {
            switch {
            case errors.Is(err, mpris.ErrLibrespotNotInstalled):
                m.ErrorMessage = "librespot not found. Install: cargo install librespot"
            case errors.Is(err, mpris.ErrSpotifyDesktopRunning):
                m.ErrorMessage = "Spotify desktop is running. Please close it first."
            default:
                m.ErrorMessage = err.Error()
            }
            return m, menuTickCmd(3 * time.Second)
        }
        m.LaunchedSpotify = true
        m.ViewState = "tui"
        return m, tea.Batch(m.pollSpotifyCmd(), m.tickCmd(), pollTickCmd())

    case ChoiceStartSpotifyDesktop:
        client, err := mpris.NewClient()
        if err == nil {
            m.MprisClient = client
            cmd := exec.Command("spotify")
            cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
            if startErr := cmd.Start(); startErr == nil {
                m.MprisClient.SetSpotifyCmd(cmd)
                m.LaunchedSpotify = true
            }
        }
        m.ViewState = "tui"
        return m, tea.Batch(m.pollSpotifyCmd(), m.tickCmd(), pollTickCmd())

    // ChoiceTUIOnly, ChoiceCheckStatus, ChoiceHelp unchanged
    }
```

### `ui/shell.go` — modulo updates

`SelectedMenuOption` wraps modulo 5 (was 4), used twice in the file. `choices` slice gains `ChoiceStartLibrespot` and `ChoiceStartSpotifyDesktop`.

## Dependency Order

1. **mpris additions** (errors + `IsSpotifyDesktopRunning` + `LaunchLibrespot`) + tests. Compiles standalone, no UI dependency.
2. **`ui/menu.go`** constant additions + view re-rendering with 5 options.
3. **`ui/shell.go`** switch cases (`ChoiceStartLibrespot`, `ChoiceStartSpotifyDesktop`), modulo-5 wrap, and modulo-5 in `MenuModel.Update` (the duplicate smaller model).
4. **`ui/shell_test.go`** update render+navigate tests + new selection tests.
5. **`main.go`** argv pre-loop + `runSetup` + helpers.

Order is mandatory: (1) unblocks (2-4). (5) is independent and can ship alongside (3).

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | `IsSpotifyDesktopRunning` true (mock `ownerName="org.mpris.MediaPlayer2.spotify"`) | existing `mockDBusConnection` |
| Unit | `IsSpotifyDesktopRunning` false (mock `ownerErr=NotFound`) | same mock |
| Unit | `LaunchLibrespot` returns `ErrLibrespotNotInstalled` when PATH empty | `t.Setenv("PATH", "")` |
| Unit | `LaunchLibrespot` returns `ErrSpotifyDesktopRunning` when owner present | mock + real `exec.LookPath` (librespot binary assumed present in CI; skip if missing) |
| Unit | Menu `View` contains new labels | `TestMenu_Render` extended |
| Unit | Menu navigation wraps at 5 | `TestMenu_Navigate` extended |
| Unit | Menu emits `ChoiceStartLibrespot` / `ChoiceStartSpotifyDesktop` | new tests |
| Manual | `--setup` OAuth happy path | documented in README + verified by maintainer on real Spotify account |
| Manual | `--setup` binary not found | `PATH="" go run . --setup` |

Coverage target for the touched files: mpris 47% → ~70%, ui 52% → ~60%.

## Threat Matrix

Subprocess + process-group boundary changes apply.

| Boundary | Adversarial case | Applicability | Design response | Planned RED test |
|---|---|---|---|---|
| Subprocess launch | `librespot` not in `PATH` | Applicable | `exec.LookPath` → `ErrLibrespotNotInstalled`, surfaced to menu / `--setup` | `TestLaunchLibrespot_BinaryNotFound` (PATH env override) |
| Subprocess launch | arg injection via `cfg`/env | Applicable | `cfg` is a filesystem path; `exec.Command` (not `bash -c`) so no shell interpolation | n/a — covered by Go stdlib contract; documented |
| Subprocess kill | wrong process group | Applicable | `Setpgid:true` set at launch; `syscall.Kill(-pid, SIGTERM)` only targets the pgid; `KillSpotify` nil-guarded | covered by existing `TestMprisClient_KillSpotify_WithProcess` |
| Subprocess kill | librespot exited before SIGTERM | Applicable | `syscall.Kill` returns `ESRCH`; `KillSpotify` swallows the error today — acceptable: process already gone, TUI exits anyway | implicit (existing test path) |
| Process integration | D-Bus owner lookup during librespot shutdown race | Applicable | `IsSpotifyDesktopRunning` is called once before launch; subsequent `IsRunning()` polls tolerate `false` (just shows "Waiting...") | covered by `IsRunning` mocks |

VCS / PR / git routing: `N/A — design does not touch git, PR automation, or shell command composition.`

Executable-file classification: `N/A — no Markdown/`.sh` execution.`

## Migration / Rollout

No data migration. No feature flag — the change is opt-in via menu choice and the `--setup` flag. Users on existing spotify-desktop installs keep working via the new menu option 5. Document in README:

- "Run `tui-spotify --setup` once before first use with librespot."
- "Menu option 1 launches librespot (OAuth cached by librespot, not us)."

Rollback: revert the change; existing D-Bus flow is preserved in option 5 and `LaunchSpotify()` is untouched.

## Open Questions

- [ ] **Spec vs. design mismatch on menu count.** `specs/librespot-backend/spec.md:60` says "Help renumbers to option 6", implying 6 options. Design prompt shows 5 options (Help stays at 4, Spotify Desktop is 5). Confirm with the user: which is canonical?
- [ ] **Config file location.** Spec accepts either `~/.config/librespot.conf` or `~/.config/tui-spotify/librespot.conf`. Design commits to the latter. Confirm.
- [ ] **`--setup` UX when stream never yields "Authenticated as".** Current design treats it as an error. Should we instead sleep + poll `IsRunning()` per the spec line 41 (60s timeout)? Recommended yes — fall back to the spec flow when no log marker appears within ~10s.
- [ ] **Cache directory creation.** `exec.Command` does not auto-create `--cache ~/.cache/librespot`. Add an `os.MkdirAll` before starting?
- [ ] **`runSetup` access to `mpris.ConfigPath()`.** Should that helper exist on `Client` to avoid hard-coding paths in `main.go`?
- [ ] **Log buffer for `bufio.Scanner`.** librespot logs can exceed the default 64KB line for very long tracebacks. Set `Buffer(max 1MB)` (snippet above does this) — confirm acceptable.
- [ ] **Set `LaunchedSpotify=false` on `ChoiceTUIOnly`?** Today it does. After option 5 launches but no player is running yet, does `pollSpotifyCmd` then auto-launch? Current pollSpotifyCmd returns `Running:false` early — no auto-launch. Good. But `killSpotifyCmd` should match by `spotifyCmd != nil`, not `LaunchedSpotify`. Confirm.
