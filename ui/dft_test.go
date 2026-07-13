package ui

import (
	"math"
	"testing"
)

func TestDFT_Compute_SineWave(t *testing.T) {
	dft := NewDFT()

	// Generate a sine wave at a frequency that should produce a peak in bin 4
	// For a 1024-sample window at 48kHz:
	// bin 4 corresponds to frequency = 4 * 48000 / 1024 ≈ 187.5 Hz
	sampleRate := 48000.0
	frequency := 187.5
	samples := make([]float32, DFTInputSize)

	for i := 0; i < DFTInputSize; i++ {
		samples[i] = float32(math.Sin(2 * math.Pi * frequency * float64(i) / sampleRate))
	}

	result := dft.Compute(samples)

	// Find the peak bin
	peakBin := 0
	peakValue := result[0]
	for i := 1; i < DFTBands; i++ {
		if result[i] > peakValue {
			peakValue = result[i]
			peakBin = i
		}
	}

	// The peak should be in bin 3, 4, or 5 (within 1 bin of expected)
	if peakBin < 3 || peakBin > 5 {
		t.Errorf("Expected peak in bins 3-5, got bin %d with value %f", peakBin, peakValue)
	}

	// Verify the peak value is significantly higher than neighboring bins
	neighborsSum := 0.0
	neighborCount := 0
	for i := 0; i < DFTBands; i++ {
		if math.Abs(float64(i-peakBin)) >= 2 {
			neighborsSum += result[i]
			neighborCount++
		}
	}
	avgNeighbor := neighborsSum / float64(neighborCount)

	if peakValue < avgNeighbor*2 {
		t.Errorf("Peak value %f should be at least 2x the average of non-neighbors %f", peakValue, avgNeighbor)
	}
}

func TestDFT_Compute_ShortInput(t *testing.T) {
	dft := NewDFT()

	// Test with short input - should return 64 zeros without panic
	shortSamples := []float32{0.1, 0.2, 0.3, 0.4}

	result := dft.Compute(shortSamples)

	if len(result) != DFTBands {
		t.Errorf("Expected result length %d, got %d", DFTBands, len(result))
	}

	// All values should be zero for short input
	for i := 0; i < DFTBands; i++ {
		if result[i] != 0 {
			t.Errorf("Expected zero at bin %d for short input, got %f", i, result[i])
		}
	}
}

func TestDFT_Compute_EmptyInput(t *testing.T) {
	dft := NewDFT()

	// Test with empty input - should return 64 zeros without panic
	result := dft.Compute([]float32{})

	if len(result) != DFTBands {
		t.Errorf("Expected result length %d, got %d", DFTBands, len(result))
	}

	// All values should be zero
	for i := 0; i < DFTBands; i++ {
		if result[i] != 0 {
			t.Errorf("Expected zero at bin %d for empty input, got %f", i, result[i])
		}
	}
}

func TestDFT_Constants(t *testing.T) {
	if DFTBands != 64 {
		t.Errorf("Expected DFTBands to be 64, got %d", DFTBands)
	}
	if DFTInputSize != 1024 {
		t.Errorf("Expected DFTInputSize to be 1024, got %d", DFTInputSize)
	}
}

func BenchmarkDFT_Compute(b *testing.B) {
	dft := NewDFT()
	samples := make([]float32, DFTInputSize)
	for i := 0; i < DFTInputSize; i++ {
		samples[i] = float32(math.Sin(2 * math.Pi * 440 * float64(i) / 48000))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dft.Compute(samples)
	}
}
