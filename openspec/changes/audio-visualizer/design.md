# Design: Audio Visualizer

## Technical Approach

Replace the sine-wave generator in `ui/visualizer.go` with a real-time spectrum analyzer. New pieces: `ui/audio.go` captures 48 kHz mono float32 PCM via `xaionaro-go/audio` into a 2048-sample ring; `ui/dft.go` runs a pure-Go O(64×1024) DFT; `ui/visualizer.go` + `ui/theme.go` consume the 64 magnitudes. Backend absent → silent fallback to procedural sine-wave.

## Architecture Decisions

| Decision | Choice | Alternatives | Why |
|---|---|---|---|
| Capture library | `xaionaro-go/audio` (PulseAudio + PortAudio side-effect imports) | CGO `portaudio`, shell `parec` | Auto-detects backend; pure-Go API surface; no external binary |
| Sample format | mono float32 LE @ 48 kHz | float64, S16LE, S32LE | Spec mandates float32; halves DFT memory vs float64 |
| Buffer size | 2048 samples (~43 ms) | 4096, 1024 | Spec mandates 2048; carries ~2 DFT frames at 30 fps |
| DFT input | 1024 samples from buffer | 2048 (full buffer) | Spec mandates 1024 input → 64 output |
| DFT algorithm | O(n²) with precomputed twiddle tables | `gonum/dsp`, FFTW via CGO | Spec forbids external FFT lib; 65 536 multiply-adds/frame ≈ 0.5 ms |
| DFT normalization | `mag/N` then `log10(1+9·m)` | raw magnitude, dBFS | Perceptual loudness curve; output 0–1 maps cleanly to bar height |
| Bin→column mapping | nearest-bin index | averaged adjacent bins | O(1); preserves transients |
| Fallback policy | silent — `AudioCapture == nil` keeps procedural mode | warn UI, error toast | Spec: "no error shown to user" |
| Threading | mutex-protected ring; DFT in Update goroutine | lock-free SPSC, channels | Mutex fine for ~43 ms window at 30 fps; simpler than lock-free |

## Data Flow

```
PulseAudio/PipeWire ──raw float32LE bytes──▶ AudioCapture.Write(p)
                                                │ bytes→float32 under mu
                                                ▼
                              [circular buf, 2048 floats]
                                                │
Visualizer.Update(playing) ──Read 1024──▶ DFT.Compute ──▶ AudioData[64]
                                                ▼
Visualizer.Render(height) ──bin→col──▶ block ramp ──▶ lipgloss styled
```

## File Changes

| File | Action | Description |
|---|---|---|
| `ui/audio.go` | Create | `AudioCapture` struct + `NewAudioCapture() (nil,nil)` on backend miss. `Write(p)` decodes PCM via `binary.LittleEndian.Uint32`+`Float32frombits`, advances `writeIdx`. `Read(out)` copies last `len(out)` samples chronologically. |
| `ui/dft.go` | Create | `DFT` with precomputed twiddle tables; `Compute(samples) []float64` returns 64 log-normalized values in [0,1]. Exports `DFTBands`, `DFTInputSize`. |
| `ui/visualizer.go` | Modify | Add `AudioCapture`, `DFT`, `AudioData`, `sampleBuf`. `Update` fills `AudioData` when capture present. `Render` maps `AudioData`→`Heights` when non-empty, else keeps procedural. Block ramp unchanged. |
| `ui/shell.go` | Modify | `NewModel` calls `NewAudioCapture`. Quit branch closes it. `renderVisualizer` picks `VisualizerBackground` (audio) vs `Visualizer` (procedural). |
| `ui/theme.go` | Modify | Add `VisualizerBackground` field; set `ANSIColor(8)` per `ui-theme` delta. |
| `go.mod` / `go.sum` | Modify | Add `github.com/xaionaro-go/audio`. |
| `ui/dft_test.go` | Create | Sine input → bin 4 dominates; benchmark < 5 ms. |
| `ui/audio_test.go` | Create | Chunked writes, wrap-around `Read`; run with `-race`. |
| `ui/visualizer_test.go` | Modify | AudioData renders `█`; nil path keeps existing assertions. |

## Interfaces / Contracts

```go
// ui/audio.go
type AudioCapture struct{ /* unexported */ }
func NewAudioCapture() (*AudioCapture, error)            // nil,nil when backend absent
func (ac *AudioCapture) Write(p []byte) (int, error)     // io.Writer — called by xaionaro-go/audio
func (ac *AudioCapture) Read(out []float32) int
func (ac *AudioCapture) Close() error

// ui/dft.go
const DFTBands, DFTInputSize = 64, 1024
type DFT struct{ /* twiddle tables */ }
func NewDFT() *DFT
func (d *DFT) Compute(samples []float32) []float64       // 64 values in [0,1]
```

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Unit (DFT) | Sine → known bin; benchmark | Synthesize samples, assert bin 4 dominates within 10×; `testing.B` regression |
| Unit (Ring) | Wrap-around, race-free | 64-byte chunked writes, chronological `Read`; `-race` |
| Unit (Visualizer) | AudioData path renders blocks; nil path keeps procedural | Assert `█` in output; extend existing `Render` tests |
| Integration | Existing shell regression | `go test ./...`; no behavioural change when audio absent |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary changed. `xaionaro-go/audio` uses CGO internally to talk to the system audio server (outside our process boundary).

## Migration / Rollout

No migration. Always-on in v1 — capture runs when backend present, silent fallback otherwise. First frame procedural, switches to spectrum once 1024 samples accumulate (~21 ms after `AudioCapture.Start`).

## Open Questions

- [ ] **Channel layout**: opened with `channels=1`. Confirm monitor source delivers summed mono, or add stereo→mono average in `Write`?
- [ ] **Height mapping**: nearest-bin (chosen) vs averaged — revisit if bars feel jittery at width > 96.
- [ ] **Pause behaviour**: keep capturing during pause (chosen — warm buffer for resume) or stop after N s idle?
- [ ] **DFT windowing**: rectangular for v1 — Hann is a follow-up if spectral leakage shows up.