# Design: Startup Menu

## Technical Approach

Single `tea.Program`, view-state discriminator. `ui.Model.ViewState` is `"menu"` or `"tui"`; `View()` and `Update()` dispatch on it. Menu is a self-contained `MenuModel` that emits `MenuChoiceMsg`; the parent consumes it, runs side effects (launch Spotify, schedule timers, show overlay), and flips `ViewState`. Timers use `tea.Tick` → `MenuTimerMsg`. Spotify launch moves out of `main.go` and out of `pollSpotifyCmd`. Menu option 1 is the ONLY caller of `exec.Command("spotify").Start()` with `SysProcAttr{Setpgid: true}`.

## Architecture Decisions

### Decision: View-state discriminator vs. nested/separate programs
**Choice**: Single `Model` with `ViewState` field; menu in `ui/menu.go` as `MenuModel` whose `Update` produces `MenuChoiceMsg` consumed by the parent.
**Alternatives**: nested `tea.Model` (indirection); separate `tea.NewProgram` (tears down alt-screen, breaks signal trap and focus).
**Rationale**: Bubble Tea's message loop handles the parent/child split natively.

### Decision: Init() returns nil in menu state
**Choice**: `Model.Init()` returns the poll batch only when `ViewState=="tui"`; else `nil`.
**Rationale**: `Init` runs once at startup; nil keeps DBus calls dormant while the user reads the menu. Transition handler re-issues the batch.

### Decision: Spotify launch via exec.Command, not D-Bus
**Choice**: Menu option 1 calls `exec.Command("spotify")` with `SysProcAttr{Setpgid: true}`, stores `*exec.Cmd` via `mpris.SetSpotifyCmd()`.
**Alternatives**: Reuse `mpris.LaunchSpotify()` (D-Bus `StartServiceByName`) — does NOT return `*exec.Cmd`, so `KillSpotify` cannot reach the spawned pgid.
**Rationale**: Process-group kill needs `Setpgid` + the local `*exec.Cmd`. `mpris.IsRunning()` is the "is Spotify up?" gate before spawn.

### Decision: --no-menu via constructor argument
**Choice**: `NewModel(viewState string) Model`; `main.go` scans `os.Args` for `--no-menu` and passes `"tui"` or `"menu"`.
**Rationale**: Constructor signature documents initial state; `main.go` keeps all I/O parsing at the edge. Avoids a race-prone `tea.Cmd` returned from Init.

### Decision: One MenuTimerMsg + state field
**Choice**: `MenuTimerMsg struct{}` (per spec); Model tracks pending action via `PendingTimer string` (`"overlay"`, `"status"`, `"help"`).
**Rationale**: Spec mandates empty struct; model state disambiguates.

### Decision: Menu `q` does NOT kill Spotify
**Choice**: `q` on menu → `tea.Quit` only; `LaunchedSpotify` stays `false` because no launch ran.
**Rationale**: `spotifyCmd != nil` guard in `mpris.Client.Close()` is the single source of truth.

## Data Flow

```
tea.Program ─KeyMsg─▶ Model.Update
                       ├─ ViewState=="menu" ─▶ MenuModel.Update ─▶ MenuChoiceMsg ─▶ Model.Update
                       │   opt 1: exec.Command + Setpgid + SetSpotifyCmd
                       │   opt 3: IsRunning + 3s TimerMsg ─▶ ViewState=tui
                       │   opt 4: 5s TimerMsg ─▶ ViewState stays "menu"
                       └─ ViewState=="tui" ─▶ lyrics/visualizer; `?` shows overlay
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `ui/menu.go` | **Create** | `MenuModel`, `MenuChoiceMsg`, constants, `NewMenuModel`, `Update`, `View` |
| `ui/shell.go` | **Modify** | Add `ViewState`, `ShowHelpOverlay`, `StatusMessage`, `PendingTimer`; branch `Init`/`View`; handle `MenuChoiceMsg`/`MenuTimerMsg`; `?` overlay; remove auto-launch in `pollSpotifyCmd` |
| `main.go` | **Modify** | Remove pre-launch; scan `os.Args` for `--no-menu`; pass viewState |
| `ui/theme.go` | **Modify** | Add `MenuTitle`, `MenuOption`, `MenuSelected`, `MenuBorder` |
| `ui/shell_test.go` | **Modify** | Add 5 menu/overlay tests |
| `mpris/client.go` | **No change** | `LaunchSpotify`, `SetSpotifyCmd`, `KillSpotify` already support the flow |

## Interfaces / Contracts

```go
// ui/menu.go
const (
    ChoiceStartSpotify = "start-spotify"
    ChoiceTUIOnly      = "tui-only"
    ChoiceCheckStatus  = "check-status"
    ChoiceHelp         = "help"
)
const menuWidth = 40

type MenuChoiceMsg struct{ Choice string }

type MenuModel struct {
    Selected int    // 0..3
    Choice   string // "" | ChoiceStartSpotify | ...
}

func NewMenuModel() MenuModel
func (m MenuModel) Init() tea.Cmd                       // nil
func (m MenuModel) Update(tea.Msg) (tea.Model, tea.Cmd)
func (m MenuModel) View() string                        // centered box, `>` on Selected

// ui/shell.go
type MenuTimerMsg struct{}                              // action from m.PendingTimer
type Model struct {
    // ... existing ...
    ViewState       string // "menu" | "tui"
    ShowHelpOverlay bool
    StatusMessage   string
    PendingTimer    string
}
func NewModel(viewState string) Model
```

## Implementation Order

1. **`ui/theme.go`** — add 4 menu colors. No behavior change.
2. **`ui/menu.go`** — full module. Self-contained; unit-test in isolation.
3. **`ui/shell.go`** — add fields; branch `Init`/`View`; wire `MenuChoiceMsg`/`MenuTimerMsg`; `?` overlay; drop `launchRetries`/`launchMaxRetries`; remove auto-launch in `pollSpotifyCmd`; update existing tests for new `NewModel` signature.
4. **`main.go`** — parse `--no-menu`; remove pre-launch; pass viewState.
5. **`ui/shell_test.go`** — append menu tests.

## Risk Mitigations

| Risk | Mitigation |
|---|---|
| `q` from menu kills pre-existing Spotify | `LaunchedSpotify` only true on option 1; `Close()` nil-guard at `mpris/client.go:101-103` |
| Pollers race menu rendering | `Init()` returns nil for `ViewState=="menu"`; restart on transition via `tea.Batch(pollSpotify, tick, pollTick)` |
| `exec.Command.Start()` failure | Spawn error → `LaunchedSpotify=false`, `ViewState="tui"`; existing "Waiting for Spotify…" branch renders |
| Timer fires after quit | `tea.Quit` drops in-flight messages; `PendingTimer` cleared on transition |
| `?` overlay blocks keyboard | Overlay consumes only the dismissing key; others flow into TUI handler in same `Update` |
| `--no-menu` hides menu by default | Default `ViewState="menu"`; flag is opt-in only |

## Verification Plan

| Piece | Verify with |
|---|---|
| `ui/theme.go` | `go test ./ui/...` stays green |
| `ui/menu.go` | `TestMenu_Render` asserts `View()` contains all 4 option labels and `>`; `TestMenu_Navigation` cycles `Selected` |
| `View()` dispatch | `NewModel("menu").View()` contains "Spt-Flow"; `NewModel("tui").View()` does not |
| `MenuChoiceMsg` handler | `TestMenu_Selection`: Enter on opt 1 → `LaunchedSpotify=true`, `ViewState="tui"` |
| `MenuTimerMsg` handler | `TestMenu_CheckStatus`: `MenuChoiceMsg{CheckStatus}` → `StatusMessage` set; `MenuTimerMsg{}` → `ViewState="tui"` |
| `?` overlay | `TestMenu_HelpOverlay`: `?` in TUI → `ShowHelpOverlay=true`; any key → `false` |
| `main.go` flag | Unit-test `hasNoMenuFlag(args)` with `--no-menu`, absent, `--other` |
| `pollSpotifyCmd` no-launch | Existing `TestShell_Update_StateMessages` still passes |

## Threat Matrix

| Boundary | Cases | Applicability | Response | RED test |
|---|---|---|---|---|
| Subprocess spawn | exec.Command("spotify") + Setpgid; spawn-failure path | Applicable | `IsRunning()` gate; spawn error → no `*exec.Cmd`, fall through | Test: `IsRunning=false` + spawn error → `ViewState="tui"`, `LaunchedSpotify=false` |
| CLI flag | `--no-menu` exact; absent; co-mingled | Applicable | Linear scan of `os.Args` | Test `hasNoMenuFlag` directly |
| Signal trap | SIGINT/SIGTERM in menu vs TUI | Applicable | Existing trap; `KillSpotify` nil-guard unchanged | Covered by `mpris/client_test.go` |
| D-Bus side effects | `LaunchSpotify` not called from `pollSpotifyCmd` | Applicable | Drop auto-launch block | Existing `TestShell_Update_StateMessages` |

## Migration / Rollout

No migration. Behavior is additive. `--no-menu` preserves the prior flow. Rollback = single revert: drop `ui/menu.go`, drop `ViewState`, restore pre-launch in `main.go`. Four new theme fields become unused (cosmetic only).

## Open Questions

- [ ] Should the menu drop fixed `menuWidth=40` and follow `m.Width`? Spec uses fixed-width centered box.
- [ ] Option 3 status: replace option list for 3s, or render as footer line?
- [ ] Should the `?` overlay consume only the dismissing key, or block all keys?
- [ ] `--no-menu` suppress signal-trap pre-registration? (Current: keep; harmless when `LaunchedSpotify=false`.)