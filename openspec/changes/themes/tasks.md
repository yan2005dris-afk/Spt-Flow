# Tasks: Themes

## Phase 1: Palette Registry

- [x] 1.1 Create `ui/themes.go` with `Themes` map, `ThemeOrder` slice.
- [x] 1.2 Add `CatppuccinMocha` constant with all 14 fields (lipgloss.Color hex).
- [x] 1.3 Add `GruvboxDark` constant with all 14 fields.
- [x] 1.4 Add `CycleTheme(current string) string` helper.

## Phase 2: Config Layer

- [x] 2.1 Create `ui/config.go` with `Config` struct (one field: `Theme`).
- [x] 2.2 Implement `LoadConfig()` — read from `~/.config/spt-flow/config.json`, fall back to `default` on missing/corrupt/unknown.
- [x] 2.3 Implement `(*Config).Save()` — atomic write (tmp + rename), log on error.
- [x] 2.4 Create `ui/config_test.go` with 6 tests (missing, corrupt, unknown, valid, round-trip, read-only).

## Phase 3: Model Theme Field

- [x] 3.1 Add `Theme *Theme` to `Model` struct in `ui/shell.go`.
- [x] 3.2 In `NewModel`, load config and set `Theme` to the active palette.
- [x] 3.3 Update `ui/themes_test.go` to verify registry integrity and `CycleTheme`.

## Phase 4: Menu Integration

- [x] 4.1 Add `ChoiceCycleTheme = "cycle-theme"` constant in `ui/menu.go`.
- [x] 4.2 Update `MenuModel.Update` enter handler to include the new option in `choices` slice.
- [x] 4.3 Add the "Theme: <name>" menu option to the rendering, between "Help / Keybindings" and "Start with Spotify Desktop".
- [x] 4.4 Add the `case ChoiceCycleTheme` in `shell.go::Update` for `MenuChoiceMsg` — cycles the theme, saves config, returns `m, nil` (no transition).
- [x] 4.5 Update `SelectedMenuOption` modulo math in shell.go to handle 6 options (was 5).

## Phase 5: Migration of DefaultTheme References

- [x] 5.1 Search and replace all `DefaultTheme.X` references in `ui/*.go` (except `ui/theme.go` and `ui/themes.go` and tests) with `m.Theme.X`.
- [x] 5.2 Verify no remaining direct references to `DefaultTheme` in the rendering path.
- [x] 5.3 Build, test, vet, gofmt.

## Phase 6: Tests

- [x] 6.1 `TestThemes_AllNamesRegistered` — every name in `ThemeOrder` is in `Themes`.
- [x] 6.2 `TestThemes_AllFieldsPopulated` — every palette has 14 valid color values.
- [x] 6.3 `TestCycleTheme` — `default` → `catppuccin-mocha` → `gruvbox-dark` → `default`.
- [x] 6.4 Update existing menu tests for the new option count (6 instead of 5).
- [x] 6.5 Update existing shell tests that reference `DefaultTheme` if needed.

## Phase 7: Documentation

- [x] 7.1 Update `README.md` with a "Themes" section: list the 3 palettes, mention the config file location, document the cycle behavior.

## Phase 8: Verification

- [x] 8.1 `go build .` clean.
- [x] 8.2 `go test ./...` all green (skip KillSpotify subprocess test).
- [x] 8.3 `go vet ./...` clean.
- [x] 8.4 `gofmt -d .` no diff.
- [x] 8.5 Manual: run, cycle through themes, verify each palette renders correctly, restart, verify persistence.

## Estimate

~350 lines across 4-5 files. Fits within the 400-line review budget (the bulk is theme color definitions, which are data, not logic).