# UI Theme Specification

## Purpose

Centralize the TUI's ANSI color palette in a single `Theme` struct, eliminating six inline `lipgloss.Color("…")` call sites in `ui/shell.go`.

---

## ADDED Requirements

### Requirement: Theme struct with 8 color fields

The system SHALL provide a `Theme` struct in `ui/theme.go` with 8 `lipgloss.ANSIColor` fields: `Header`, `Footer`, `LyricActive`, `LyricInactive`, `LyricPlain`, `Visualizer`, `Error`, `Waiting`.

### Requirement: DefaultTheme variable

The system SHALL expose a package-level `DefaultTheme` variable with these ANSI values:
- Header: `"15"`
- Footer: `"12"`
- LyricActive: `"12"`
- LyricInactive: `"15"`
- LyricPlain: `"7"`
- Visualizer: `"8"`
- Error: `"9"`
- Waiting: `"7"`

### Requirement: Color accessor methods

The system MUST replace all 6 hardcoded `lipgloss.Color("…")` call sites in `ui/shell.go` with `DefaultTheme.*` accessor references.

**Replacements:**
| Old | New |
|-----|-----|
| `lipgloss.Color("15")` in `renderHeader` | `DefaultTheme.Header` |
| `lipgloss.Color("12")` in `renderFooter` | `DefaultTheme.Footer` |
| `lipgloss.Color("12")` active lyric in `renderLyrics` | `DefaultTheme.LyricActive` |
| `lipgloss.Color("15")` inactive lyric in `renderLyrics` | `DefaultTheme.LyricInactive` |
| `lipgloss.Color("7")` plain lyrics in `renderLyrics` | `DefaultTheme.LyricPlain` |
| `lipgloss.Color("8")` in `renderVisualizer` | `DefaultTheme.Visualizer` |

### Requirement: Error color on lyric display

The system MUST apply `DefaultTheme.Error` (color `"9"`, bright red) when rendering the lyric area and `m.ErrorMessage != ""`.

### Requirement: Waiting color in footer

The system MUST apply `DefaultTheme.Waiting` (color `"7"`, gray) to the waiting indicator when Spotify is not running.

### Requirement: Theme is package-level, not on Model

The `Model` struct SHALL NOT carry a `Theme` field. Theme is accessed as a package-level `DefaultTheme` variable directly inside render functions.

---

## Scenarios

### Scenario: Default colors on startup

- GIVEN the application starts with `DefaultTheme`
- WHEN the UI renders header, footer, lyrics, and visualizer
- THEN all elements use their assigned colors from `DefaultTheme`

### Scenario: Error message renders in red

- GIVEN `m.ErrorMessage != ""`
- WHEN `renderLyrics` is called
- THEN the lyric area renders the error text styled with `DefaultTheme.Error` (color `"9"`)

### Scenario: Waiting for Spotify renders in gray

- GIVEN Spotify is not running
- WHEN `renderFooter` or the waiting screen renders
- THEN the waiting indicator uses `DefaultTheme.Waiting` (color `"7"`)
