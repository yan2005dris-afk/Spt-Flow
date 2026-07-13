# Audio Visualizer Specification

## Purpose

Replace the procedural sine-wave visualizer with a real-time audio spectrum analyzer that captures PCM audio from PulseAudio/PipeWire and renders frequency bars as a transparent background behind lyrics.

---

## ADDED Requirements

### Requirement: AudioCapture struct

The system SHALL provide an `AudioCapture` struct in `ui/audio.go` that manages a goroutine reading PCM chunks from the system audio backend.

The struct MUST:
- Use `github.com/xaionaro-go/audio` for cross-platform audio capture
- Auto-detect PulseAudio or PipeWire backend
- Capture mono audio at 48kHz sample rate, PCM float32 format
- Fill a circular buffer of 2048 samples (≈43ms of audio)
- Provide `Read(samples []float32) int` method that copies latest samples into the provided slice and returns the number of samples copied
- Provide `Close()` method to stop the goroutine and release resources
- Initialize to `nil` (not an error) if no audio backend is available on startup
- Expose `NewAudioCapture() (*AudioCapture, error)` constructor that returns `nil, nil` if no audio backend is available

### Requirement: DFT spectrum analysis

The system SHALL provide a `DFT` struct in `ui/dft.go` that computes a 64-bin frequency spectrum from audio samples.

The struct MUST:
- Maintain exactly 64 frequency bins
- Provide `Compute(samples []float32) []float64` that applies a pure-Go O(n²) DFT over 1024 samples
- Return 64 normalized magnitude values in range 0.0 to 1.0
- Represent logarithmically-spaced frequency bands from low (bass) to high (treble)
- Use NO external FFT library — pure Go implementation

### Requirement: Visualizer audio integration

The `Visualizer` struct in `ui/visualizer.go` SHALL be extended with:
- `AudioCapture *AudioCapture` field
- `DFT *DFT` field
- `AudioData []float64` field (64 magnitudes from last DFT compute)

The `Update(playing bool)` method MUST:
- If `AudioCapture != nil`: read samples from circular buffer → compute DFT → store magnitudes in `AudioData`
- If `AudioCapture == nil`: generate procedural waveform and set `AudioData` to all zeros
- Drive bar heights from `AudioData`

The `Render(height int)` method MUST:
- Use `AudioData` (64 magnitudes) to set bar heights
- If `AudioData` has all zeros or is empty: render minimal or no bars
- Use `DefaultTheme.VisualizerBackground` color for the bars
- Apply lower opacity effect via the muted ANSI color

### Requirement: Shell integration

The `ui/shell.go` `NewModel` function SHALL:
- Initialize `AudioCapture` via `NewAudioCapture()`
- If `NewAudioCapture` returns `nil`, proceed silently with nil

The `Init()` method SHALL:
- If `AudioCapture != nil`, the visualizer starts receiving audio data automatically

The visualizer display SHALL only show bars when `AudioData` has non-zero magnitudes.

### Requirement: VisualizerBackground theme color

The `Theme` struct in `ui/theme.go` SHALL add:
- `VisualizerBackground lipgloss.ANSIColor` field

The `DefaultTheme` SHALL set:
- `VisualizerBackground: lipgloss.Color("8")` (dark gray — muted, does not compete with lyrics)

---

## Scenarios

### Scenario: App starts with audio backend available

- GIVEN the system has PulseAudio or PipeWire running
- WHEN the application starts
- THEN `NewAudioCapture()` returns a non-nil `AudioCapture`
- AND the visualizer shows real frequency bars when music plays

### Scenario: App starts without audio backend

- GIVEN the system has no PulseAudio/PipeWire available
- WHEN the application starts
- THEN `NewAudioCapture()` returns `nil, nil`
- AND the visualizer uses the procedural sine-wave fallback
- AND no error is shown to the user

### Scenario: Music plays with audio capture active

- GIVEN `AudioCapture != nil` and music is playing
- WHEN `Update(true)` is called
- THEN samples are read from the circular buffer
- AND DFT computes 64 magnitude values
- AND `AudioData` drives the visualizer bar heights
- AND bars react to the actual frequency spectrum

### Scenario: Music paused with audio capture

- GIVEN `AudioCapture != nil` and music is paused
- WHEN `Update(false)` is called
- THEN bars decay to zero using the existing decay behavior

### Scenario: Audio capture thread crashes

- GIVEN `AudioCapture` was initialized and running
- WHEN the capture goroutine crashes or returns an error
- THEN `AudioCapture` is set to `nil`
- AND the visualizer silently falls back to procedural mode
- AND no warning or error UI is shown

### Scenario: No lyrics displayed

- GIVEN no lyrics are available
- WHEN the visualizer renders
- THEN bars still react to audio
- AND the visualizer occupies the full bottom area

### Scenario: AudioData all zeros renders minimal bars

- GIVEN `AudioData` contains all zeros
- WHEN `Render(height)` is called
- THEN no bars or very small bars are rendered
- AND the lyrics (if any) remain clearly visible

---

## Technical Constraints

- Circular buffer: 2048 samples, monaural, 48kHz, float32
- DFT input: 1024 samples from the circular buffer
- DFT output: 64 bands, normalized 0.0–1.0
- Update rate: every ~33ms (30fps) driven by existing TickMsg timer
- No external FFT library — pure Go DFT implementation
- Silent fallback: no UI warning when audio capture is unavailable
