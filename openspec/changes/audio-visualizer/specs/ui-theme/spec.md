# Delta for ui-theme

## MODIFIED Requirements

### Requirement: Theme struct with 8 color fields

(Previously: Theme struct with 8 color fields)

The system SHALL provide a `Theme` struct in `ui/theme.go` with 9 `lipgloss.ANSIColor` fields: `Header`, `Footer`, `LyricActive`, `LyricInactive`, `LyricPlain`, `Visualizer`, `Error`, `Waiting`, `VisualizerBackground`.

### Requirement: DefaultTheme variable

(Previously: DefaultTheme variable with 8 colors)

The system SHALL expose a package-level `DefaultTheme` variable with these ANSI values:
- Header: `"15"`
- Footer: `"12"`
- LyricActive: `"12"`
- LyricInactive: `"15"`
- LyricPlain: `"7"`
- Visualizer: `"8"`
- VisualizerBackground: `"8"`
- Error: `"9"`
- Waiting: `"7"`

---

## ADDED Requirements

### Requirement: VisualizerBackground color for transparent overlay

The system SHALL provide `VisualizerBackground` (ANSI color `"8"`, dark gray) for rendering visualizer bars in transparent background mode.

This color MUST be used when the visualizer renders bars driven by real audio data, providing a muted appearance that does not compete with lyrics visibility.

---

## Scenarios

### Scenario: Default colors on startup

- GIVEN the application starts with `DefaultTheme`
- WHEN the UI renders header, footer, lyrics, and visualizer
- THEN all elements use their assigned colors from `DefaultTheme`
- AND `VisualizerBackground` is available for transparent overlay rendering

### Scenario: Error message renders in red

- GIVEN `m.ErrorMessage != ""`
- WHEN `renderLyrics` is called
- THEN the lyric area renders the error text styled with `DefaultTheme.Error` (color `"9"`)

### Scenario: Waiting for Spotify renders in gray

- GIVEN Spotify is not running
- WHEN `renderFooter` or the waiting screen renders
- THEN the waiting indicator uses `DefaultTheme.Waiting` (color `"7"`)

### Scenario: VisualizerBackground used for audio-driven bars

- GIVEN real audio capture is active and music is playing
- WHEN the visualizer renders frequency bars
- THEN the bars use `DefaultTheme.VisualizerBackground` (color `"8"`, dark gray)
- AND the bars appear muted behind the lyrics
