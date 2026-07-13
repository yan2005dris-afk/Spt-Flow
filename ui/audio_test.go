package ui

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestAudioCapture_WriteRead(t *testing.T) {
	// Create AudioCapture with known buffer size
	ac := &AudioCapture{
		buffer: make([]float32, 2048),
		size:   2048,
	}

	// Write some samples using proper IEEE 754 representation
	samples := []float32{1.0, 2.0, 3.0, 4.0, 5.0}
	for _, s := range samples {
		bits := math.Float32bits(s)
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, bits)
		ac.Write(bytes)
	}

	// Read them back
	out := make([]float32, 5)
	n := ac.Read(out)

	if n != 5 {
		t.Errorf("Expected to read 5 samples, got %d", n)
	}

	for i := 0; i < 5; i++ {
		if out[i] != samples[i] {
			t.Errorf("Expected sample %f at index %d, got %f", samples[i], i, out[i])
		}
	}
}

func TestAudioCapture_WrapAround(t *testing.T) {
	// Create AudioCapture with small buffer for testing wrap-around
	ac := &AudioCapture{
		buffer: make([]float32, 10),
		size:   10,
	}

	// Fill the buffer completely (write 10 samples)
	for i := 0; i < 10; i++ {
		bits := math.Float32bits(float32(i))
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, bits)
		ac.Write(bytes)
	}

	// Read 5 samples - should get first 5
	out := make([]float32, 5)
	n := ac.Read(out)
	if n != 5 {
		t.Errorf("Expected to read 5 samples, got %d", n)
	}

	// Verify the first 5 samples are 0-4
	for i := 0; i < 5; i++ {
		if out[i] != float32(i) {
			t.Errorf("Expected %f at index %d, got %f", float32(i), i, out[i])
		}
	}

	// Write 3 more samples (wrapping around)
	for i := 0; i < 3; i++ {
		bits := math.Float32bits(float32(100 + i))
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, bits)
		ac.Write(bytes)
	}

	// Read 5 samples - should get remaining 5 from first batch (5,6,7,8,9)
	out = make([]float32, 5)
	n = ac.Read(out)
	if n != 5 {
		t.Errorf("Expected to read 5 samples, got %d", n)
	}

	// Should get samples 5,6,7,8,9
	for i := 0; i < 5; i++ {
		expected := float32(i + 5)
		if out[i] != expected {
			t.Errorf("Expected %f at index %d, got %f", expected, i, out[i])
		}
	}
}

func TestAudioCapture_OverwriteOldSamples(t *testing.T) {
	// Test that writing more samples than buffer size overwrites old samples
	ac := &AudioCapture{
		buffer: make([]float32, 10),
		size:   10,
	}

	// Write 10 samples: [0,1,2,3,4,5,6,7,8,9]
	for i := 0; i < 10; i++ {
		bits := math.Float32bits(float32(i))
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, bits)
		ac.Write(bytes)
	}

	// Write 5 more samples (overwriting first 5: 0,1,2,3,4)
	// After overwriting: [100,101,102,103,104,5,6,7,8,9]
	for i := 0; i < 5; i++ {
		bits := math.Float32bits(float32(100 + i))
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, bits)
		ac.Write(bytes)
	}

	// Read all 10 samples - should get [5,6,7,8,9,100,101,102,103,104]
	// (the oldest 5 were overwritten, so we get 5-9 then 100-104)
	out := make([]float32, 10)
	n := ac.Read(out)
	if n != 10 {
		t.Errorf("Expected to read 10 samples, got %d", n)
	}

	// Should get samples 100,101,102,103,104,5,6,7,8,9 (first 5 were overwritten)
	expected := []float32{100, 101, 102, 103, 104, 5, 6, 7, 8, 9}
	for i := 0; i < 10; i++ {
		if out[i] != expected[i] {
			t.Errorf("Expected %f at index %d, got %f", expected[i], i, out[i])
		}
	}
}

func TestAudioCapture_ReadFewerThanAvailable(t *testing.T) {
	ac := &AudioCapture{
		buffer: make([]float32, 2048),
		size:   2048,
	}

	// Write 10 samples
	for i := 0; i < 10; i++ {
		bits := math.Float32bits(float32(i))
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, bits)
		ac.Write(bytes)
	}

	// Read only 3
	out := make([]float32, 3)
	n := ac.Read(out)
	if n != 3 {
		t.Errorf("Expected to read 3 samples, got %d", n)
	}

	// Should get first 3 samples (0, 1, 2)
	for i := 0; i < 3; i++ {
		if out[i] != float32(i) {
			t.Errorf("Expected %f at index %d, got %f", float32(i), i, out[i])
		}
	}

	// Read 3 more - should get 3, 4, 5
	n = ac.Read(out)
	if n != 3 {
		t.Errorf("Expected to read 3 samples, got %d", n)
	}

	for i := 0; i < 3; i++ {
		expected := float32(i + 3)
		if out[i] != expected {
			t.Errorf("Expected %f at index %d, got %f", expected, i, out[i])
		}
	}
}

func TestAudioCapture_Close(t *testing.T) {
	ac := &AudioCapture{
		buffer: make([]float32, 2048),
		size:   2048,
		closed: false,
	}

	// Close should work without panic
	err := ac.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	if !ac.closed {
		t.Error("Expected closed to be true after Close()")
	}
}

func TestAudioCapture_ReadAfterClose(t *testing.T) {
	ac := &AudioCapture{
		buffer: make([]float32, 2048),
		size:   2048,
	}
	ac.Close()

	// Read after close should return 0
	out := make([]float32, 10)
	n := ac.Read(out)
	if n != 0 {
		t.Errorf("Expected 0 after read on closed buffer, got %d", n)
	}
}

func TestAudioCapture_BufferSize(t *testing.T) {
	ac := &AudioCapture{
		buffer: make([]float32, 2048),
		size:   2048,
	}

	if ac.BufferSize() != 2048 {
		t.Errorf("Expected BufferSize to be 2048, got %d", ac.BufferSize())
	}
}
