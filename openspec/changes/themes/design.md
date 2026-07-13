# Design: Themes

## Architecture

```
+--------------------+
|   ui/themes.go     |   <-- new: palette registry
|   - Themes map     |
|   - ThemeOrder     |
|   - 3 palettes     |
+--------+-----------+
         |  lookup by name
         v
+--------------------+
|     ui/config.go   |   <-- new: persistence
|   - Load()         |
|   - Save()         |
+--------+-----------+
         |
         v
+--------------------+
|   ui/menu.go       |   modified: cycle option
+--------+-----------+
         |
         v
+--------------------+
|   ui/shell.go      |   modified: Model.Theme Theme
|   - View()         |   reads m.Theme.X
+--------------------+
```

## Palette Definitions

Catppuccin Mocha (https://github.com/catppuccin/catppuccin):

| Slot | Color | Hex |
| --- | --- | --- |
| Header | `text` | `#cdd6f4` |
| Footer | `blue` | `#89b4fa` |
| LyricActive | `mauve` | `#cba6f7` |
| LyricInactive | `text` | `#cdd6f4` |
| LyricPlain | `subtext1` | `#a6adc8` |
| Visualizer | `green` | `#a6e3a1` |
| VisualizerBackground | `yellow` | `#f9e2af` |
| Error | `red` | `#f38ba8` |
| Waiting | `subtext0` | `#a6adc8` |
| MenuTitle | `mauve` | `#cba6f7` |
| MenuOption | `text` | `#cdd6f4` |
| MenuSelected | `peach` | `#fab387` |
| MenuDim | `surface2` | `#585b70` |
| MenuBorder | `surface1` | `#45475a` |

Gruvbox Dark (https://github.com/gruvbox/gruvbox):

| Slot | Color | Hex |
| --- | --- | --- |
| Header | `fg` | `#ebdbb2` |
| Footer | `blue` | `#83a598` |
| LyricActive | `purple` | `#d3869b` |
| LyricInactive | `fg` | `#ebdbb2` |
| LyricPlain | `gray` | `#a89984` |
| Visualizer | `green` | `#b8bb26` |
| VisualizerBackground | `yellow` | `#fabd2f` |
| Error | `red` | `#fb4934` |
| Waiting | `gray` | `#a89984` |
| MenuTitle | `yellow` | `#fabd2f` |
| MenuOption | `fg` | `#ebdbb2` |
| MenuSelected | `orange` | `#fe8019` |
| MenuDim | `bg3` | `#665c54` |
| MenuBorder | `bg2` | `#504945` |

`default` keeps the existing `lipgloss.ANSIColor(N)` values for backward compatibility.

## File Layout

```
ui/
  theme.go        # Theme struct + DefaultTheme (existing, untouched in v1)
  themes.go       # NEW: palette registry, 3 palette constants
  themes_test.go  # NEW
  config.go       # NEW: load/save config to ~/.config/spt-flow/config.json
  config_test.go  # NEW
  menu.go         # MODIFIED: theme option + cycle handler
  shell.go        # MODIFIED: Model.Theme Theme, all DefaultTheme refs → m.Theme
  other files     # MODIFIED: replace DefaultTheme with m.Theme
```

## Type Design

```go
// ui/themes.go

var Themes = map[string]Theme{
    "default":          DefaultTheme,
    "catppuccin-mocha": CatppuccinMocha,
    "gruvbox-dark":     GruvboxDark,
}

var ThemeOrder = []string{"default", "catppuccin-mocha", "gruvbox-dark"}

// CatppuccinMocha is the Catppuccin Mocha palette.
var CatppuccinMocha = Theme{
    Header:               lipgloss.Color("#cdd6f4"),
    Footer:               lipgloss.Color("#89b4fa"),
    // ... etc.
}

// GruvboxDark is the Gruvbox dark palette.
var GruvboxDark = Theme{
    Header: lipgloss.Color("#ebdbb2"),
    // ... etc.
}

// CycleTheme returns the next palette name in ThemeOrder, wrapping.
func CycleTheme(current string) string {
    for i, name := range ThemeOrder {
        if name == current {
            return ThemeOrder[(i+1)%len(ThemeOrder)]
        }
    }
    return ThemeOrder[0] // fallback to default
}
```

## Config Layer

```go
// ui/config.go

type Config struct {
    Theme string `json:"theme"`
}

const defaultConfigPath = "spt-flow/config.json"

func LoadConfig() (*Config, error) {
    base, err := os.UserConfigDir()
    if err != nil {
        return &Config{Theme: "default"}, nil
    }
    path := filepath.Join(base, defaultConfigPath)

    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return &Config{Theme: "default"}, nil
        }
        // Any other read error: log and fall through to default.
        fmt.Fprintf(os.Stderr, "config: read failed: %v\n", err)
        return &Config{Theme: "default"}, nil
    }

    var c Config
    if err := json.Unmarshal(data, &c); err != nil {
        fmt.Fprintf(os.Stderr, "config: corrupt file, using default: %v\n", err)
        return &Config{Theme: "default"}, nil
    }

    if _, ok := Themes[c.Theme]; !ok {
        return &Config{Theme: "default"}, nil
    }
    return &c, nil
}

func (c *Config) Save() error {
    base, err := os.UserConfigDir()
    if err != nil {
        return err
    }
    dir := filepath.Join(base, "spt-flow")
    if err := os.MkdirAll(dir, 0700); err != nil {
        return err
    }
    path := filepath.Join(dir, "config.json")

    // Atomic write: tmp + rename
    tmp, err := os.CreateTemp(dir, "config-*.json.tmp")
    if err != nil {
        return err
    }
    tmpName := tmp.Name()
    defer os.Remove(tmpName)

    enc := json.NewEncoder(tmp)
    enc.SetIndent("", "  ")
    if err := enc.Encode(c); err != nil {
        tmp.Close()
        return err
    }
    if err := tmp.Sync(); err != nil {
        tmp.Close()
        return err
    }
    if err := tmp.Close(); err != nil {
        return err
    }
    return os.Rename(tmpName, path)
}
```

## Model Changes

```go
// ui/shell.go

type Model struct {
    // ... existing fields ...
    Theme Theme // value for live swap; initialized by NewModel
}
```

`NewModel` initializes `Theme` from the loaded config:

```go
func NewModel(viewState string) Model {
    cfg, _ := LoadConfig()
    if _, ok := Themes[cfg.Theme]; !ok {
        cfg.Theme = "default"
    }
    theme := Themes[cfg.Theme]
    return Model{
        // ... existing fields ...
        Theme: theme,
    }
}
```

## Menu Option

The current menu has 5 options. We add a 6th: `Theme: <name>`.

Index layout (after the change):
- 0: Start with Librespot + TUI
- 1: Open TUI only
- 2: Check Spotify status
- 3: **Theme: <name>** (NEW)
- 4: Help / Keybindings
- 5: Start with Spotify Desktop

The `enter` key on option 3 cycles the theme. The current `MenuChoiceMsg` carries a string choice. We add `ChoiceCycleTheme = "cycle-theme"`.

When `MenuChoiceMsg{Choice: ChoiceCycleTheme}` arrives, the model's `Update` handler:
1. Mutates `m.Theme` to the next palette.
2. Saves the config.
3. Stays in the menu (does NOT transition to TUI).

The `Update` for `MenuChoiceMsg` already switches on `msg.Choice` — we add a case for `ChoiceCycleTheme` that does the cycle and returns `m, nil` (no transition).

## Rendering Migration

All `DefaultTheme.X` references in `ui/menu.go`, `ui/shell.go`, `ui/visualizer.go` (if any) become `m.Theme.X`. The `renderMenuView` function takes `m Model` already, so this is a search-and-replace.

`DefaultTheme` itself stays as-is in `theme.go` — it's the v0 "default" palette definition.

## Test Strategy

`ui/themes_test.go`:
- `TestThemes_AllNamesRegistered` — every name in `ThemeOrder` exists in `Themes`.
- `TestThemes_AllFieldsPopulated` — every palette has 14 non-zero color slots.
- `TestCycleTheme` — cycling `default` → `catppuccin-mocha` → `gruvbox-dark` → `default`.

`ui/config_test.go`:
- `TestLoadConfig_Missing` — no file, returns `default`.
- `TestLoadConfig_Corrupt` — garbage JSON, returns `default`, logs.
- `TestLoadConfig_UnknownTheme` — `{"theme": "bogus"}`, returns `default`.
- `TestLoadConfig_Valid` — `{"theme": "gruvbox-dark"}`, returns it.
- `TestConfig_Save` — round-trips through Save → Load.
- `TestConfig_Save_ReadOnly` — dir read-only, Save fails gracefully (logs), in-memory state still has the change.

## Open Questions

None. The approach is direct and follows the same pattern as the lyrics cache.