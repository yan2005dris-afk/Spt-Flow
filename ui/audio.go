package ui

import (
	"context"
	"encoding/binary"
	"io"
	"math"
	"sync"
)

// AudioCapture manages a circular buffer of audio samples captured from the system.
// It implements io.Writer to receive PCM data from xaionaro-go/audio.
type AudioCapture struct {
	mu       sync.Mutex
	buffer   []float32
	size     int
	writeIdx int
	readIdx  int
	full     bool // true when buffer has size elements (readIdx == writeIdx && full)
	closed   bool

	// xaionaro-go/audio integration
	stream io.Closer
}

// NewAudioCapture initializes a new AudioCapture with a 2048-sample circular buffer.
// Returns (nil, nil) if no audio backend is available.
func NewAudioCapture(ctx context.Context) (*AudioCapture, error) {
	ac := &AudioCapture{
		buffer: make([]float32, 2048),
		size:   2048,
	}

	// Attempt to start audio capture via xaionaro-go/audio
	if err := ac.startCapture(ctx); err != nil {
		// Silent fallback - return nil AudioCapture
		return nil, nil
	}

	return ac, nil
}

// startCapture tries to start audio capture using xaionaro-go/audio.
// Returns an error if no backend is available.
func (ac *AudioCapture) startCapture(ctx context.Context) error {
	// This is a stub implementation.
	// The xaionaro-go/audio library would be used here to capture audio.
	// For now, we return an error to trigger the fallback.
	//
	// TODO: Implement full xaionaro-go/audio integration when needed.
	// The integration would look like:
	//   recorder := audio.NewRecorderAuto(ctx)
	//   stream, err := recorder.RecordPCM(ctx, 48000, 1, audio.PCMFormatFloat32LE, ac)
	//   if err != nil {
	//       return err
	//   }
	//   ac.stream = stream
	return io.EOF // Signals that capture isn't available
}

// Write implements io.Writer. It decodes PCM float32LE bytes and stores
// samples in the circular buffer. When the buffer is full, new samples
// overwrite the oldest samples.
func (ac *AudioCapture) Write(p []byte) (int, error) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if ac.closed {
		return 0, io.EOF
	}

	// Each sample is 4 bytes (float32)
	sampleCount := len(p) / 4

	for i := 0; i < sampleCount; i++ {
		bits := binary.LittleEndian.Uint32(p[i*4 : i*4+4])
		sample := math.Float32frombits(bits)

		ac.buffer[ac.writeIdx] = sample
		ac.writeIdx = (ac.writeIdx + 1) % ac.size

		// If we wrapped around and caught up to readIdx, we overwrote oldest data
		if ac.writeIdx == ac.readIdx {
			ac.full = true
		}
	}

	return len(p), nil
}

// Read copies the latest len(out) samples into out, in chronological order.
// Returns the number of samples copied.
func (ac *AudioCapture) Read(out []float32) int {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if ac.closed {
		return 0
	}

	// Calculate how many samples are available
	var available int
	if ac.full {
		// Buffer is full - all size samples are available
		available = ac.size
	} else if ac.writeIdx >= ac.readIdx {
		available = ac.writeIdx - ac.readIdx
	} else {
		available = ac.size - ac.readIdx + ac.writeIdx
	}

	toRead := len(out)
	if toRead > available {
		toRead = available
	}

	// Copy samples in chronological order (oldest first)
	for i := 0; i < toRead; i++ {
		idx := (ac.readIdx + i) % ac.size
		out[i] = ac.buffer[idx]
	}

	// Advance read index
	ac.readIdx = (ac.readIdx + toRead) % ac.size

	// If we just caught up to writeIdx, buffer is no longer full
	if ac.full && ac.readIdx == ac.writeIdx {
		ac.full = false
	}

	return toRead
}

// Close stops the audio capture and releases resources.
func (ac *AudioCapture) Close() error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.closed = true

	if ac.stream != nil {
		ac.stream.Close()
		ac.stream = nil
	}

	return nil
}

// BufferSize returns the size of the circular buffer.
func (ac *AudioCapture) BufferSize() int {
	return ac.size
}
