# Startup Menu Specification

## Purpose

Pre-TUI selection screen letting the user choose how to interact with Spotify before the main TUI renders.

## ADDED Requirements

### Requirement: ViewState field on Model

`ui.Model` SHALL have a `ViewState string` field with values `"menu"` or `"tui"`. Default: `"menu"`.

### Requirement: Menu renders instead of TUI when ViewState is "menu"

When `m.ViewState == "menu"`, `View()` SHALL render the menu screen and MUST NOT render the lyrics/TUI view.

### Requirement: Menu screen layout

The menu SHALL render as a centered box with:
- Title: `"Spt-Flow"` in `MenuTitle` color
- Four numbered options listed 1–4
- Footer: `"↑↓ navigate · Enter select · q quit"` in `MenuBorder` color
- Selected option in `MenuSelected` style (Bold); unselected in `MenuUnselected` style

### Requirement: Menu options

The menu SHALL offer four options:
1. `"Start Spotify + TUI"` — launches Spotify then transitions to TUI
2. `"Open TUI only"` — transitions to TUI without launching Spotify
3. `"Check Spotify status"` — shows running/not-running status for 3s, then transitions to TUI
4. `"Help / Keybindings"` — shows keybindings table for 5s, then returns to menu

### Requirement: Menu navigation

The menu SHALL allow navigation via:
- Arrow keys `↑`/`↓` or `j`/`k` to move selection
- `Enter` to confirm the selected option
- `q` to quit the application (MUST NOT kill Spotify if not launched by the TUI)

### Requirement: Transition to TUI on option 1

When option 1 is selected, the system SHALL:
1. Call `mpris.IsRunning()`. If false, call `mpris.LaunchSpotify()` via `exec.Command("spotify").Start()` with `SysProcAttr{Setpgid: true}` and store the `*exec.Cmd` on the MPRIS client.
2. Set `m.ViewState = "tui"` and `m.LaunchedSpotify = true`
3. Re-initialize the model (reset `LastUpdated`, `Visualizer`, restart pollers)

### Requirement: Transition to TUI on option 2

When option 2 is selected, the system SHALL set `m.ViewState = "tui"` and `LaunchedSpotify = false`, then re-initialize the model.

### Requirement: Check status (option 3)

When option 3 is selected, the system SHALL:
1. Call `mpris.IsRunning()` and display the result as a status message
2. Wait 3 seconds
3. Set `m.ViewState = "tui"` and `LaunchedSpotify = false` (MUST NOT set `LaunchedSpotify = true`)
4. Re-initialize the model

### Requirement: Help (option 4)

When option 4 is selected, the system SHALL display the keybindings table for 5 seconds, then return to the menu (ViewState remains `"menu"`).

### Requirement: TUI renders when ViewState is "tui"

When `m.ViewState == "tui"`, `View()` SHALL render the normal TUI (lyrics, visualizer, footer).

### Requirement: ? keybindings overlay in TUI

When `m.ViewState == "tui"` and the user presses `?`, the system SHALL show the keybindings overlay and return to TUI on any keypress. The overlay MUST NOT change `ViewState`.

### Requirement: --no-menu CLI flag

The application SHALL accept a `--no-menu` flag. When provided, startup MUST skip the menu and go directly to TUI (`ViewState = "tui"` on Init).

## Scenarios

1. **App starts → menu appears** — GIVEN app starts → WHEN Init runs → THEN `ViewState = "menu"`, menu renders.
2. **Arrow keys navigate** — GIVEN menu displayed → WHEN user presses `↓` → THEN second option is selected.
3. **Enter on option 1 → Spotify + TUI** — GIVEN Spotify not running → WHEN user selects option 1 → THEN Spotify launches, `ViewState = "tui"`, TUI renders.
4. **Enter on option 2 → TUI only** — GIVEN any state → WHEN user selects option 2 → THEN `ViewState = "tui"`, `LaunchedSpotify = false`, TUI renders.
5. **Enter on option 3 → status then TUI** — GIVEN any state → WHEN user selects option 3 → THEN status shown 3s, `ViewState = "tui"`, `LaunchedSpotify = false`.
6. **Enter on option 4 → keybindings then menu** — GIVEN any state → WHEN user selects option 4 → THEN keybindings shown 5s, returns to menu.
7. **q on menu → quit** — GIVEN menu displayed → WHEN user presses `q` → THEN app exits, Spotify not killed.
8. **--no-menu → TUI directly** — GIVEN `--no-menu` flag → WHEN Init runs → THEN `ViewState = "tui"` on startup, TUI renders immediately.
9. **? in TUI → overlay** — GIVEN `ViewState = "tui"` → WHEN user presses `?` → THEN keybindings overlay shown, any key returns to TUI.
