# Tasks: Startup Menu

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~250 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | single-pr |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Full startup menu feature | PR 1 | `go test ./ui/...` | Manual: run TUI, verify menu renders | Drop ui/menu.go, remove ViewState from shell.go |

## Phase 1: Foundation - Theme Colors

- [x] 1.1 Add `MenuTitle`, `MenuOption`, `MenuSelected`, `MenuDim` fields to `Theme` struct in `ui/theme.go`
- [x] 1.2 Set defaults: MenuTitle="15" (white), MenuOption="7" (gray), MenuSelected="12" (cyan), MenuDim="8" (dark gray)

## Phase 2: Core Implementation - Menu Module

- [x] 2.1 Create `ui/menu.go` with constants: `ChoiceStartSpotify`, `ChoiceTUIOnly`, `ChoiceCheckStatus`, `ChoiceHelp`
- [x] 2.2 Add message types: `MenuChoiceMsg{Choice string}`, `MenuTimerMsg{}` (add to shell.go or new msg.go)
- [x] 2.3 Create `MenuModel` struct with `Selected int`, `Width int`, `Height int`
- [x] 2.4 Implement `NewMenuModel(height, width int) MenuModel`
- [x] 2.5 Implement `MenuModel.Update()`: handle j/k/up/down (wrap 0-3), Enter emits `MenuChoiceMsg`, q emits `tea.Quit`
- [x] 2.6 Implement `MenuModel.View()`: responsive centered box, title "Spt-Flow" + subtitle, options with ">" indicator, footer with navigation hints

## Phase 3: Integration - Shell Integration

- [x] 3.1 Add `ViewState` ("menu"|"tui"), `SelectedMenuOption`, `MenuWidth`, `ShowHelpOverlay`, `StatusMessage`, `PendingTimer` fields to `Model` in `ui/shell.go`
- [x] 3.2 Rename `NewModel()` to `NewModel(viewState string)` and set `ViewState` from parameter
- [x] 3.3 Modify `Init()`: return nil when `ViewState=="menu"`, otherwise return poll batch
- [x] 3.4 Modify `View()`: if `ViewState=="menu"`, call `renderMenuView(m)` helper
- [x] 3.5 Create `renderMenuView(m Model) string` helper in `ui/menu.go` (not a separate tea.Model)
- [x] 3.6 Modify `Update()`: handle `tea.WindowSizeMsg` to set `m.Width`, `m.Height`, `m.MenuWidth`
- [x] 3.7 Handle `MenuChoiceMsg`: "start-spotify"→exec.Command+Setpgid+SetSpotifyCmd+ViewState="tui"; "tui-only"→ViewState="tui"; "check-status"→IsRunning()+show status in footer+3s timer; "help"→show overlay 5s
- [x] 3.8 Handle `MenuTimerMsg`: transition ViewState="tui" or dismiss help overlay
- [x] 3.9 Handle `?` key in TUI state: set `ShowHelpOverlay=true`
- [x] 3.10 Handle any key when `ShowHelpOverlay==true`: set `ShowHelpOverlay=false`
- [x] 3.11 Remove auto-launch code from `pollSpotifyCmd` (the block that calls LaunchSpotify)

## Phase 4: Main Entry Point

- [x] 4.1 Parse `os.Args` in `main.go` for `--no-menu` flag
- [x] 4.2 Pass initial `ViewState` to `NewModel()`: "menu" or "tui" based on flag
- [x] 4.3 Remove Spotify pre-launch code from `main.go` (lines 27-34)

## Phase 5: Testing

- [x] 5.1 Add `TestMenu_RenderMenuView`: create Model with ViewState="menu", call View(), verify title and options
- [x] 5.2 Add `TestMenu_Navigation`: press j key, SelectedMenuOption increments; wrap at 3→0
- [x] 5.3 Add `TestMenu_OverlayToggles`: ViewState="tui", ? key shows overlay, any key dismisses
- [x] 5.4 Add `TestMenu_CheckStatusTransition`: MenuChoiceMsg "check-status" sets timer, timer fires → ViewState="tui"
- [x] 5.5 Update existing tests for new `NewModel` signature

## Phase 6: Verification

- [x] 6.1 `go build .` compiles without errors
- [x] 6.2 `go test ./...` passes
- [x] 6.3 `go vet ./...` passes
- [x] 6.4 `gofmt -d .` shows no diff

(End of file - total 71 lines)
