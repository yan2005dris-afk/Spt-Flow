# Spec: Themes

## Requirements

### REQ-1: Built-in Palettes

Three palettes MUST be defined:
- `default` — current `DefaultTheme` (preserves backward compatibility).
- `catppuccin-mocha` — Catppuccin Mocha palette.
- `gruvbox-dark` — Gruvbox dark palette.

Each palette MUST define all 14 color slots in the `Theme` struct:
Header, Footer, LyricActive, LyricInactive, LyricPlain, Visualizer, VisualizerBackground, Error, Waiting, MenuTitle, MenuOption, MenuSelected, MenuDim, MenuBorder.

### REQ-2: Palette Registry

A registry `map[string]Theme` MUST be exported as `Themes`. The order in `ThemeOrder []string` MUST list `default` first and be used to cycle through palettes in the menu.

### REQ-3: Color Definition

Each palette MUST use `lipgloss.Color("#hex")` so we get the exact source color, not a 16-color approximation. The `default` palette MAY keep its current `lipgloss.ANSIColor(N)` values for backward compatibility — converting it is a non-functional change and not required by v1.

### REQ-4: Config File Location

User choice MUST be persisted at `$XDG_CONFIG_HOME/spt-flow/config.json`. If `XDG_CONFIG_HOME` is unset, fall back to `~/.config/spt-flow/config.json`. The directory MUST be created with mode `0700` if it doesn't exist.

### REQ-5: Config File Format

```json
{ "theme": "<palette-name>" }
```

The file MAY contain other keys in the future; v1 only reads `theme`. Unknown fields MUST be preserved on save.

### REQ-6: Atomic Config Writes

Writes MUST be atomic (tmp file + rename), same pattern as the lyrics cache. A crash mid-write MUST NOT corrupt the existing config.

### REQ-7: Load Order

On startup, the TUI MUST attempt to read the config. If the file is missing, empty, corrupt, or contains an unknown theme name, the TUI MUST fall back to `default`. There MUST be no error visible to the user in any of these cases.

### REQ-8: Menu Integration

The startup menu MUST show a new option labelled "Theme: <name>" where `<name>` is the currently active palette. The option MUST be inserted between "Help / Keybindings" and "Start with Spotify Desktop" (i.e. as the 4th option, index 3, before the existing 5th).

When the user presses Enter on the theme option:
1. The active palette MUST cycle to the next one in `ThemeOrder` (wrapping back to `default` after the last).
2. The choice MUST be persisted to disk immediately.
3. The menu MUST re-render with the new palette applied (live preview).
4. The TUI MUST NOT exit the menu; the user can keep navigating.

### REQ-9: Apply on TUI Entry

When the user picks any non-theme option and enters the TUI proper, the TUI MUST use the currently active palette. The `View()` method MUST read colors from the model's active theme pointer, NOT from `DefaultTheme` directly.

### REQ-10: All Lipgloss Calls Migrated

Every `lipgloss.ANSIColor` / `lipgloss.Color` reference in `ui/*.go` that previously pointed to `DefaultTheme.X` MUST now point to `m.Theme.X` (or equivalent). There MUST be no remaining direct references to `DefaultTheme` in the rendering path. (Tests may still reference `DefaultTheme` for the `default` palette definition.)

## Scenarios

### SCN-1: First-time startup (no config)

Given the config file does not exist
When the TUI starts
Then the menu renders with the `default` palette
And no warning is shown to the user.

### SCN-2: Valid saved choice

Given the config file contains `{"theme": "catppuccin-mocha"}`
When the TUI starts
Then the menu and TUI render in Catppuccin Mocha.

### SCN-3: Corrupt config file

Given the config file contains invalid JSON
When the TUI starts
Then a warning is logged to stderr
And the menu renders with the `default` palette
And the file is overwritten on the next theme change.

### SCN-4: Unknown theme name

Given the config file contains `{"theme": "synthwave"}`
When the TUI starts
Then the `default` palette is used
And no error is shown.

### SCN-5: Cycle to next palette

Given the active palette is `default`
When the user selects "Theme: default" and presses Enter
Then the active palette becomes `catppuccin-mocha`
And the menu re-renders with Catppuccin colors
And the config file is updated to `{"theme": "catppuccin-mocha"}`.

### SCN-6: Cycle wraps around

Given the active palette is `gruvbox-dark`
When the user cycles the theme
Then the active palette becomes `default` (wrap-around).

### SCN-7: Theme persists across restarts

Given the user set the theme to `catppuccin-mocha` in session 1
When the TUI is restarted
Then the menu and TUI render in Catppuccin Mocha immediately.

### SCN-8: Palette round-trip

Given a palette name `catppuccin-mocha`
When the code looks up `Themes["catppuccin-mocha"]`
Then a `Theme` value is returned
And all 14 fields are populated with valid `lipgloss.Color` / `lipgloss.ANSIColor` values.

### SCN-9: Atomic write under failure

Given the config directory is read-only
When the user cycles the theme
Then an error is logged to stderr
And the in-memory active palette is still updated
And on next start, the user gets the palette that was last persisted (could be the previous one).