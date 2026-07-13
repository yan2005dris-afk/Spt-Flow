# Proposal: Startup Menu

## Intent

`main.go` currently auto-launches Spotify (if not running), starts the TUI, and shows "Waiting for Spotify…" when Spotify isn't reachable. The user gets no choice: they always land in the TUI whether Spotify is up or not, with no opportunity to inspect keybindings or check Spotify status without launching the full TUI. This change adds a pre-TUI startup menu that lets the user pick between launching Spotify + TUI, opening the TUI only, checking Spotify status, or viewing keybindings.

## Scope

### In Scope

- New `ui/menu.go` exporting `MenuModel` (`Init`/`Update`/`View`) and a `MenuChoiceMsg` carrying one of `StartSpotifyTUI`, `OpenTUI`, `CheckStatus`, `ShowHelp`.
- Render the menu as a centered, rounded-border lipgloss box: title, subtitle, four numbered options with selection highlight.
- Selection via number keys (`1`–`4`) and arrow keys (`↑`/`↓` + `enter`).
- Add `ViewState string` field to `ui.Model` (`"menu"` | `"tui"`). `Init()` returns `nil` when state is `"menu"`; pollers only start on transition to `"tui"`.
- Modify `ui.shell.View()` to render the menu when `viewState == "menu"`, the lyrics screen when `"tui"`.
- Modify `ui.shell.Update` to handle `MenuChoiceMsg` and apply the selection.
- Modify `main.go` to remove the pre-launch Spotify step (now driven by menu choice 1) and start the program in `"menu"` state.
- Add four menu-specific colors to `ui/theme.go`: `MenuTitle`, `MenuSelected`, `MenuUnselected`, `MenuBorder`.
- Keybindings shown in option 4 are view-only and exit without entering the TUI.

### Out of Scope

- Configurable menu options or persistence of the last selection.
- Spotify playback controls inside the menu.
- Rewriting `LaunchSpotify`/`KillSpotify` (reused as-is from the previous change).
- A TUI-internal `?` help overlay (startup-only for v1).

## Capabilities

### New Capabilities

- `startup-menu`: pre-TUI selection screen with four options, transitions to the TUI view state on selection.

### Modified Capabilities

- `spotify-lifecycle`: the TUI no longer auto-launches Spotify at program start. Launch is triggered only on menu option 1 (or option 3 if Spotify isn't running). The `LaunchedSpotify` flag and `KillSpotify` semantics are preserved unchanged.

## Approach

**View state, not separate program** (recommended). Add a `ViewState` discriminator to the existing `ui.Model`. The menu is a render branch of `View()` while `viewState == "menu"`. On selection, flip `viewState = "tui"` and seed the model with the same init/commands as today. One `tea.NewProgram`, one alt-screen, one signal trap, one terminal session.

**Why not a separate program**: a second `tea.NewProgram` tears down and rebuilds the alt-screen, loses keyboard focus, and forces signal-handling re-wiring. Keeping the menu in the same model preserves the bubble tea message loop and lipgloss pipeline continuously — no flash, no handoff.

**Selection flow**:
1. `1` → `viewState = "tui"`; trigger `LaunchSpotify` if `!IsRunning()`.
2. `2` → `viewState = "tui"`; TUI begins in "Waiting…" state if Spotify is absent.
3. `3` → run `IsRunning()` check, render result for ~2 s, then transition to `"tui"`.
4. `4` → render keybindings list, then `tea.Quit` (TUI never opens).

**Rendering**: `lipgloss.Place` to center the box inside `m.Width × m.Height`. Selected option is `Bold(true) + DefaultTheme.MenuSelected`; others `DefaultTheme.MenuUnselected`; border in `DefaultTheme.MenuBorder`; title in `DefaultTheme.MenuTitle`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `ui/menu.go` | New | `MenuModel`, `MenuChoiceMsg`, `MenuChoice` constants, `renderMenu` |
| `ui/shell.go` | Modified | `ViewState` field; branching in `Init`/`Update`/`View`; `MenuChoiceMsg` handler |
| `main.go` | Modified | Remove pre-launch; start program in `"menu"` state; signal trap unchanged |
| `ui/theme.go` | Modified | Add `MenuTitle`, `MenuSelected`, `MenuUnselected`, `MenuBorder` ANSI fields |
| `ui/shell_test.go` | Modified | New test: menu selection flips `ViewState` to `"tui"` |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| `Init()` runs pollers before user selects an option, defeating the menu | Med | `Init()` returns `nil` when `viewState == "menu"`; pollers only kick off on transition |
| Menu causes a visible flash on transition | Low | Same alt-screen, no clear; first paint after selection is the TUI view directly |
| `KillSpotify` runs on user-pre-existing Spotify after menu option 2 | Low | Preserve the `LaunchedSpotify` guard; only option 1 flips it `true` |
| User presses `q` from the menu → cleanup still runs Spotify kill | Low | Same guard; flag stays `false` on menu-only exits |
| Two transitions (menu→tui) break existing tick/polling state | Med | Reset `LastUpdated`, `Visualizer`, and re-issue pollers in the transition handler |

## Rollback Plan

Single revert. Remove `ui/menu.go`; drop `ViewState` from `ui.Model` (compiler forces it). `main.go` restores the pre-launch step. No schema, no migration. The four new `ui/theme.go` fields become unused (cosmetic only).

## Dependencies

- `github.com/charmbracelet/bubbletea v1.3.10` (already in `go.mod`) — `tea.Model` interface unchanged.
- `github.com/charmbracelet/lipgloss v1.1.0` (already in `go.mod`) — used for the centered, bordered menu box.

## Success Criteria

- [ ] `ui/menu.go` exports `MenuModel` and `MenuChoiceMsg`; `MenuChoice` constants cover the four options.
- [ ] `ui.Model.ViewState == "menu"` on startup; flips to `"tui"` only after a valid selection.
- [ ] Number keys `1`–`4` select directly; arrow keys + `enter` navigate and confirm.
- [ ] Option 1 launches Spotify only when not already running, then enters the TUI.
- [ ] Option 2 enters the TUI without launching Spotify.
- [ ] Option 3 surfaces Spotify running/not-running status, then enters the TUI.
- [ ] Option 4 shows the keybindings list and exits without entering the TUI.
- [ ] `SIGINT`/`SIGTERM` during or after the menu kills Spotify only when the user chose option 1.
- [ ] `ui/theme.go` exposes `MenuTitle`, `MenuSelected`, `MenuUnselected`, `MenuBorder` with documented ANSI defaults.
- [ ] `go test ./...` passes including a new test asserting the menu→tui transition.
- [ ] Forecasted diff ≤ ~250 added lines across 4 files (well under the 400-line review budget).

## Open Questions

1. Should option 3 transition into the TUI after showing status, or quit? Proposal assumes transition (consistent with option 2).
2. Should option 1's label change to "Open TUI" when Spotify is already running, to communicate the no-op? Proposal: keep the same label; behavior is idempotent.
3. Should a CLI flag (`--no-menu`) skip the menu for power users who always want the TUI? Proposal: yes, trivial to add during apply; not blocking the proposal.
4. Should the menu remember the last choice (e.g., `~/.config/tui-spotify/state.json`) and skip itself next launch? Proposal: no — keeps v1 simple; can layer in later.
5. Should option 4 be reachable from inside the TUI as a `?` overlay as well, or strictly startup-only? Proposal: startup-only for v1; TUI-internal help deferred.
