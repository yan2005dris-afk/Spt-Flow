# Proposal: Audio Visualizer

## Intent

`ui/visualizer.go` currently synthesizes a sine-wave pattern that has no relation to the music actually playing. The bars look "alive" but they lie — a user watching the TUI while Spotify plays a quiet acoustic track sees the same hypnotic wave they'd see during a bass-heavy drop. This change replaces the fake visualizer with a real-time spectrum analyzer that captures system audio from PulseAudio/PipeWire, runs an in-house DFT, and renders the result as a transparent overlay behind the lyrics.

The user-visible promise: when music is playing, the bars dance to *that* music; when the backend is unavailable, the existing sine-wave fallback kicks in so the screen is never empty.

## Scope

### In Scope

- Add `github.com/xaionaro-go/audio` dependency for PulseAudio/PipeWire monitor-source capture; the library auto-selects the backend.
- New `ui/audio.go`: capture goroutine + thread-safe ring buffer (1024 PCM samples, mono Float64 at 48 kHz).
- New `ui/dft.go`: pure-Go O(n²) DFT producing exactly 64 magnitude bins per 1024-sample frame.
- Refactor `ui/visualizer.go`: replace sine-wave generator with spectrum-driven heights; add transparent-overlay mode (low-opacity color, block-character ramp unchanged).
- Detect backend at startup (`pactl` / `pw-cli` probe); on capture failure, fall back to procedural visualizer silently.
- `ui/shell.go`: render visualizer as a background layer behind lyrics (smaller band when lyrics are visible, taller band when lyrics are absent).
- `ui/theme.go`: add `VisualizerBackground` color (muted, low-opacity feel via ANSI gray-8 dim variant).
- Tests: `ui/dft_test.go` (sine-in, known-bins-out), `ui/audio_test.go` (ring buffer wrap), `ui/visualizer_test.go` extended for spectrum input + transparent mode.

### Out of Scope

- Lyrics rendering changes (positioning handled by existing `ui/shell.go` logic).
- MPRIS / `mpris/` package changes.
- Lyrics sync, lyrics-plain, or lyrics-search changes.
- New startup menu entries (toggle will be a later change; v1 always-on with silent fallback).
- Microphone capture (monitor source only).
- External FFT library — pure-Go DFT is intentional.

## Capabilities

### New Capabilities

- `audio-visualizer`: real-time spectrum-driven visualizer with PulseAudio/PipeWire capture, automatic fallback to procedural mode, transparent background rendering behind lyrics.

### Modified Capabilities

- `ui-theme`: add a `VisualizerBackground` ANSI color for the overlay variant.

## Approach

**Capture** (`ui/audio.go`). One goroutine opens `xaionaro-go/audio` pointed at the default monitor source, converts incoming PCM to mono `Float64` at 48 kHz, and writes into a lock-free ring buffer of size 1024. The goroutine starts lazily inside `Visualizer.Update` the first time `playing == true` and exits when paused for >5s (no churn on pause/resume). On any backend error, the capture goroutine logs once, sets `fallback = true`, and never retries.

**Spectrum** (`ui/dft.go`). `ComputeSpectrum(samples [1024]float64) [64]float64` performs a textbook DFT: for each bin `k ∈ [0,64)`, sum `Σ samples[i] * (cos(2πki/N) − i·sin(2πki/N))`. Window: rectangular for v1 (sufficient for the visual); magnitude normalized by `N/4`. At 48 kHz with 1024 samples per frame, this is ~131k multiply-adds per frame, ~0.5 ms on commodity CPUs — trivial at 30 fps.

**Visualizer** (`ui/visualizer.go`). `Visualizer` gains `Heights [64]float64` (replacing the variable-width `Heights []float64`) and `Transparent bool`. `Update(playing bool, spectrum [64]float64)` either feeds spectrum bins into a smoothed height array (exponential decay between frames for visual continuity) or runs the existing sine-wave fallback when `spectrum == nil` or `fallback == true`. `Render` keeps the existing block-character ramp; the foreground color is set by `lipgloss.NewStyle().Foreground(DefaultTheme.VisualizerBackground)` in transparent mode.

**Shell integration** (`ui/shell.go`). `renderVisualizer` becomes a background layer. When lyrics are present, the visualizer occupies the bottom 20% of the viewport; when lyrics are absent, it occupies the bottom 50%. Width remains the viewport width (spectrum is downsampled to width by averaging adjacent bins). Existing `m.Visualizer.Update(true/false)` call sites gain a second argument for the spectrum slice.

**Fallback**. Silent. No warning, no log spam, no UI change. The user only notices the absence of audio reactivity, not the absence of audio capture.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `ui/audio.go` | New | `AudioCapture` goroutine, ring buffer, backend detection (`pactl`/`pw-cli`), `SpectrumSink` interface |
| `ui/dft.go` | New | `ComputeSpectrum([1024]float64) [64]float64` |
| `ui/visualizer.go` | Modified | `Heights` becomes `[64]float64`; `Update(playing, spectrum)`; `Transparent` mode; sine-wave fallback path |
| `ui/shell.go` | Modified | Wire capture → DFT → visualizer; transparent background rendering behind lyrics; resize heights by lyrics presence |
| `ui/theme.go` | Modified | Add `VisualizerBackground lipgloss.ANSIColor` (gray-8 dim) |
| `go.mod` / `go.sum` | Modified | Add `github.com/xaionaro-go/audio` |
| `ui/dft_test.go` | New | DFT correctness: synthesized sine → expected bin magnitudes |
| `ui/audio_test.go` | New | Ring buffer wrap, concurrent read/write race-free |
| `ui/visualizer_test.go` | Modified | Spectrum-driven update path + transparent flag coverage |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| PulseAudio/PipeWire unavailable on user's machine (headless, WSL, no audio server) | Med | Silent fallback to procedural visualizer; capture failure logged once at WARN, then dropped |
| Goroutine leak on pause/resume churn | Low | Capture goroutine self-terminates after 5s of inactivity; `tea.Msg`-driven ticker already gates `Update` |
| DFT is O(n²) — too slow on very old CPUs | Low | 1024×64 = 65k multiplies per frame, ~0.5 ms; benchmark in `dft_test.go` to fail CI if regression |
| `xaionaro-go/audio` dependency adds binary size | Low | Library is small (no CGO); accepted cost |
| Visualizer overshadows lyrics when audio is loud | Med | Transparent overlay uses dimmed color (gray-8 vs current gray-15) + bottom-band placement when lyrics present |
| First-frame latency (capture → first spectrum) | Low | Procedural mode renders for the first ~1 second while the buffer fills, then transitions smoothly |

## Rollback Plan

Single revert. `go.mod` reverts via `git checkout`; `ui/audio.go` and `ui/dft.go` are new files (delete). `ui/visualizer.go` reverts to the sine-wave `Heights []float64`. `ui/shell.go` reverts to the existing single-arg `Update(playing)` callsite. No schema, no migration, no persistent state. Worst case: delete the two new files and run `go mod tidy`.

## Dependencies

- `github.com/xaionaro-go/audio` — new; pure Go, no CGO, auto-selects PulseAudio/PipeWire/ALSA backends.
- `github.com/charmbracelet/bubbletea v1.3.10` — unchanged; existing `tea.Msg` tickers drive the update loop.
- `github.com/charmbracelet/lipgloss v1.1.0` — unchanged; used for the overlay style.

## Success Criteria

- [ ] `ui/audio.go` exposes `AudioCapture` with `Start()`, `Stop()`, `Latest() [1024]float64`, `Fallback() bool`.
- [ ] `ui/dft.go` exposes `ComputeSpectrum` and a passing unit test for a synthesized sine input.
- [ ] `Visualizer.Update(playing, spectrum)` produces reactive bars from `spectrum`; falls back to sine wave when `spectrum == nil` or `Fallback() == true`.
- [ ] `pactl info` / `pw-cli info` probes correctly identify available backend before `Start()`.
- [ ] Visualizer renders behind lyrics when lyrics are present, larger when absent.
- [ ] No goroutine leaks under continuous pause/resume (verified by `goleak` or `runtime.NumGoroutine()` snapshot in test).
- [ ] `go test ./...` passes; new tests add ≥80% coverage to `ui/audio.go` and `ui/dft.go`.
- [ ] Manual smoke test on a Linux desktop with PipeWire: visualizer reacts to actual system audio within ~1s.
- [ ] Forecasted diff ≤ ~350 added / ~80 removed lines across 9 files (under the 400-line review budget).

## Open Questions

1. **Color**: keep the existing `Visualizer` (gray-8) for the procedural mode and use a separate `VisualizerBackground` (dimmer gray-8 or gray-0 italic) only when real audio is active? Proposal assumes one color per mode; deferring final palette to design phase.
2. **Default visibility**: should the visualizer be on by default, or gated behind a future menu toggle / config flag? Proposal assumes on by default with silent fallback; user can disable in a later change if needed.
3. **Failure visibility**: silent fallback vs one-time toast/log? Proposal: silent, WARN log only — keeps the screen clean, surfaces in `--debug` if added later.
4. **Sample rate**: 48 kHz is a Spotify/PipeWire default; should we auto-detect via the monitor source's native rate and resample in Go? Proposal: assume 48 kHz for v1; revisit if real-world capture reports a different rate.
5. **DFT windowing**: rectangular (current proposal) vs Hann/Hamming (cleaner bins, +0.5 ms). Proposal: rectangular for v1 to keep the DFT budget trivial; window upgrade is a follow-up if spectral leakage becomes visible.
