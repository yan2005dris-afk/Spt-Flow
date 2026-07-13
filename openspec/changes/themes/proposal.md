# Proposal: Theme Switching

## Intent

The TUI is locked to a single color scheme — what ships in `DefaultTheme`. Users who like Gruvbox, Catppuccin, Nord, or any other palette have no way to use one without editing the source and rebuilding. This change ships three hand-picked palettes and a way to switch between them from the menu.

The user-visible promise: pick a palette from the startup menu, and the entire TUI (header, lyrics, visualizer, menu, help overlay, footer) renders in that palette from that point forward. Choice persists across runs.

## Scope

### In Scope

- Three built-in palettes:
  - `default` — current `DefaultTheme` (ANSI 16 colors, dark).
  - `catppuccin-mocha` — Catppuccin Mocha, the most popular TUI palette in 2024-2025.
  - `gruvbox-dark` — Gruvbox dark, the other top TUI palette.
- New file `ui/themes.go` with a registry of palettes indexed by name.
- New file `ui/config.go` for reading/writing the user's choice to `~/.config/spt-flow/config.json`.
- New menu entry: **"Theme: <name>"** at index 3, immediately before "Help / Keybindings". Selecting it cycles to the next palette and re-renders the menu.
- The active palette name is shown next to the menu option (e.g. "Theme: catppuccin-mocha ›"). Arrow keys still navigate; pressing Enter on the theme option cycles forward.
- Apply-on-TUI-entry: the menu itself shows the palette live (so users see the new colors as they cycle). When they pick another menu option and enter the TUI, the TUI also uses the chosen palette.
- Persist the choice to disk on every change. On next start, load it and apply before the first render.
- Tests: `ui/themes_test.go` (palette name round-trip, default fallback), `ui/config_test.go` (read/write/empty/corrupt config).

### Out of Scope

- **Per-component color customization** — fixed palettes only.
- **Light themes** — Gruvbox Light, Catppuccin Latte. Could be added later; v1 is dark only.
- **Theme preview** — a small swatch shown in the menu. Not needed: the menu itself uses the new palette, which is preview enough.
- **Hotkey to cycle theme from TUI** — the user explicitly asked for menu-based, not keybinding.
- **Custom user-defined palettes** — could come later via `~/.config/spt-flow/themes/<name>.json`; v1 ships only the 3 built-ins.
- **Auto-detect terminal background** — could be a heuristic; v1 is explicit choice only.
- **TOML/YAML config** — JSON is enough for one key. If we add more later, revisit.

## Approach

Themes are constants, not user-tunable. The registry is a `map[string]Theme` and `DefaultTheme` keeps its current role (the "default" name lookup falls back to it).

```go
var Themes = map[string]Theme{
    "default":           DefaultTheme,
    "catppuccin-mocha":  CatppuccinMocha,
    "gruvbox-dark":      GruvboxDark,
}

var ThemeOrder = []string{"default", "catppuccin-mocha", "gruvbox-dark"}
```

The active theme lives on the `Model` as a `Theme` value. All `lipgloss` calls in the codebase currently reference `DefaultTheme.X` — those need to be replaced with `m.Theme.X` (or whatever the field is called). Since the existing code is small and the change is mechanical, we can do this in one pass.

Config is a single-key JSON file:
```json
{ "theme": "catppuccin-mocha" }
```

Load order:
1. Read `~/.config/spt-flow/config.json` on startup.
2. If file missing or corrupt → use `default`.
3. If `theme` key missing or value is not in the registry → use `default`.
4. Set the model's active theme.

Save: on theme change in the menu, write the config file atomically (same tmp+rename pattern as the lyrics cache).