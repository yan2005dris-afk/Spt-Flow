# Tasks: Audio Visualizer

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~400-500 |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | ask-on-risk |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: single-pr
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Theme + DFT module | PR 1 | `go test ./ui -run DFT` | N/A - unit test | ui/dft.go, ui/theme.go |
| 2 | Audio capture + integration | PR 1 | `go test ./ui -run Audio` | N/A - unit test | ui/audio.go |
| 3 | Visualizer + shell wiring | PR 1 | `go test ./ui -run Visualizer` | N/A - unit test | ui/visualizer.go, ui/shell.go |

## Phase 1: Theme additions

- [x] 1.1 Add `VisualizerBackground lipgloss.ANSIColor` field to `Theme` struct in `ui/theme.go`
- [x] 1.2 Set `VisualizerBackground: lipgloss.ANSIColor(8)` in `DefaultTheme` in `ui/theme.go`
- [x] 1.3 Add test: verify `DefaultTheme.VisualizerBackground` is set to expected value in `ui/theme_test.go` (create if needed)

## Phase 2: DFT module

- [x] 2.1 Create `ui/dft.go` with `DFT` struct, `NewDFT()`, `Compute()` methods; export `DFTBands=64`, `DFTInputSize=1024`
- [x] 2.2 Create `ui/dft_test.go`: test `Compute` with known sine wave input (verify peak in expected bin)
- [x] 2.3 Create `ui/dft_test.go`: test `Compute` with short input (< 1024) returns 64 zeros without panic

## Phase 3: Audio capture module

- [x] 3.1 Create `ui/audio.go` with `AudioCapture` struct, circular buffer (2048 samples), mutex protection
- [x] 3.2 Implement `Write(p []byte)` to decode float32LE PCM bytes and write to circular buffer
- [x] 3.3 Implement `Read(out []float32)` to copy latest samples in chronological order
- [x] 3.4 Implement `NewAudioCapture(ctx context.Context)` and `Close()` methods
- [x] 3.5 Add `github.com/xaionaro-go/audio` to `go.mod` and run `go mod tidy`
- [x] 3.6 Stub audio backend: if `xaionaro-go/audio` API doesn't match design, return nil AudioCapture for silent fallback

## Phase 4: Visualizer integration

- [x] 4.1 Modify `ui/visualizer.go`: add `AudioCapture`, `DFT`, `AudioData`, `sampleBuf` fields to `Visualizer` struct
- [x] 4.2 Modify `Update(playing bool)`: read 1024 samples from AudioCapture → DFT.Compute() → store in AudioData; if AudioCapture nil, keep procedural behavior
- [x] 4.3 Modify `Render(height int)`: use AudioData for bar heights when non-empty; use VisualizerBackground color for audio mode, Visualizer for procedural mode

## Phase 5: Shell integration

- [x] 5.1 Modify `ui/shell.go`: in `NewModel(viewState string)`, create AudioCapture with context, initialize DFT and buffers
- [x] 5.2 Modify `ui/shell.go`: in quit path (q/ctrl+c), call AudioCapture.Close() before Quit
- [x] 5.3 Verify no changes needed to `Init()` method

## Phase 6: Tests

- [x] 6.1 Create `ui/audio_test.go`: test circular buffer wrap-around (write more than buffer size, verify Read returns correct order)
- [x] 6.2 Create `ui/audio_test.go`: test Read with fewer samples available than requested
- [x] 6.3 Run `go test -race ./ui` to verify no race conditions

## Phase 7: Verification

- [x] 7.1 Run `go build ./...` — must compile
- [x] 7.2 Run `go test ./...` — all pass
- [x] 7.3 Run `go vet ./...` — clean
- [x] 7.4 Run `gofmt -d .` — no diff
- [x] 7.5 Run `go mod tidy` — clean dependencies

## Implementation Notes

- **Silent fallback**: if `NewAudioCapture()` returns error or nil, visualizer uses procedural path automatically
- **Capture continues when paused**: buffer keeps filling even when Spotify is paused (keeps data ready for resume)
- **Nearest bin mapping**: bin index = column index (preserves transients, no averaging)
- **Rectangular windowing**: no Hann window for v1 (simpler, preserves transients)
- **Thread safety**: mutex-protected circular buffer; DFT runs in Update goroutine (called from tea tick)

(End of file - total 79 lines)