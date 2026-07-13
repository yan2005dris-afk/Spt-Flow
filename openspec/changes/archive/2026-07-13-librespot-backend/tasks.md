# Tasks: librespot-backend

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~300 |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | single-pr |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: N/A
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | mpris additions (errors, IsSpotifyDesktopRunning, LaunchLibrespot, ConfigPath, CacheDir) | PR 1 | `go test ./mpris -v` | Manual: select menu option 1 | mpris/client.go additions only |
| 2 | main.go --setup flag and runSetup() | PR 1 | `go build . && ./tui-spotify --setup` | Manual: run --setup with real Spotify | main.go changes only |
| 3 | ui/menu.go 5 options | PR 1 | `go test ./ui -v` | Manual: view menu | ui/menu.go only |
| 4 | ui/shell.go menu handlers and pollSpotifyCmd | PR 1 | `go test ./ui -v` | Manual: select each menu option | ui/shell.go only |
| 5 | Integration verification | PR 1 | `go build . && go test ./... && go vet ./...` | Full build + test | Full change |

## Phase 1: mpris/client.go additions

- [x] 1.1 Add error vars `ErrLibrespotNotInstalled` and `ErrSpotifyDesktopRunning` to mpris/client.go
- [x] 1.2 Add `IsSpotifyDesktopRunning() bool` method - call GetNameOwner("org.mpris.MediaPlayer2.spotify"), return true if owner present
- [x] 1.3 Add `ConfigPath() string` helper returning `~/.config/tui-spotify/librespot.conf`
- [x] 1.4 Add `CacheDir() string` helper returning `~/.cache/librespot/`
- [x] 1.5 Add `LaunchLibrespot(configPath string) error` method:
  - Check exec.Lookup("librespot") → ErrLibrespotNotInstalled if not found
  - Check IsSpotifyDesktopRunning() → ErrSpotifyDesktopRunning if true
  - Build args: --name "Spt-Flow", --enable-oauth, --cache CacheDir(), optionally --config configPath
  - Use exec.Command with SysProcAttr{Setpgid: true}
  - Store via SetSpotifyCmd(cmd)
- [x] 1.6 Add test: `TestIsSpotifyDesktopRunning_True` (mock returns owner name)
- [x] 1.7 Add test: `TestIsSpotifyDesktopRunning_False` (mock returns empty/no owner)
- [x] 1.8 Add test: `TestLaunchLibrespot_BinaryNotFound` - verify ErrLibrespotNotInstalled
- [x] 1.9 Add test: `TestLaunchLibrespot_DesktopRunning` - verify ErrSpotifyDesktopRunning

## Phase 2: main.go changes

- [x] 2.1 In main(), parse os.Args for `--setup` flag before client creation
- [x] 2.2 If `--setup` flag present: call runSetup() and exit appropriately
- [x] 2.3 Add `printInstallInstructions()` function that prints install commands for cargo/pacman
- [x] 2.4 Implement `runSetup()` function:
  - Check binary via exec.Lookup("librespot")
  - Ensure cache dir exists via os.MkdirAll
  - Build command with --name, --enable-oauth, --cache, --config
  - Use bufio.Scanner with 1MB buffer on stdout
  - Scan for OAuth URL and "Authenticated as" / "Connected to AP" markers
  - Wait with 60 second timeout using select/chan pattern
- [x] 2.5 Add imports: bufio, bytes, io, path/filepath, strings, time

## Phase 3: ui/menu.go changes

- [x] 3.1 Add menu constants: ChoiceStartLibrespot = "start-librespot", ChoiceStartSpotifyDesktop = "start-spotify-desktop"
- [x] 3.2 Update choices slice to 5 options:
  - "Start with Librespot + TUI"
  - "Open TUI only"
  - "Check Spotify status"
  - "Help / Keybindings"
  - "Start with Spotify Desktop"
- [x] 3.3 Update renderMenuView to display 5 options with correct navigation text
- [x] 3.4 Update MenuModel.Update modulo from 4 to 5

## Phase 4: ui/shell.go changes

- [x] 4.1 Modify pollSpotifyCmd() to NOT call LaunchSpotify() - just return SpotifyStateMsg{Running: false}
- [x] 4.2 Add MenuChoiceMsg handler for ChoiceStartLibrespot:
  - Create mpris client, check IsSpotifyDesktopRunning()
  - Call LaunchLibrespot with ConfigPath()
  - Handle errors: set m.ErrorMessage and stay in menu
- [x] 4.3 Add MenuChoiceMsg handler for ChoiceStartSpotifyDesktop (existing LaunchSpotify logic)
- [x] 4.4 Update SelectedMenuOption modulo from 4 to 5
- [x] 4.5 Update choices slice in Update and key handler to 5 options

## Phase 5: Testing and verification

- [x] 5.1 Run `go build .` - must compile without errors
- [x] 5.2 Run `go test ./...` - all tests must pass
- [x] 5.3 Run `go vet ./...` - must be clean
- [x] 5.4 Run `gofmt -d .` - no diff needed
- [ ] 5.5 Manual test: `./tui-spotify --setup` (requires Spotify account)
- [ ] 5.6 Manual test: select menu option 1 (Start with Librespot + TUI)
- [ ] 5.7 Manual test: select menu option 5 (Start with Spotify Desktop)

## Implementation Notes

- ConfigPath(): add to mpris/client.go or create mpris/config.go
- runSetup(): in main.go
- Scanner buffer: 1MB (1024*1024 bytes)
- KillSpotify check: use `spotifyCmd != nil` (not LaunchedSpotify)
- 60 second timeout for OAuth flow
