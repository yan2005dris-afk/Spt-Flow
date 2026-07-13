package ui

import "math"

const (
	DFTBands     = 64
	DFTInputSize = 1024
)

// DFT implements a pure-Go O(n²) Discrete Fourier Transform
// for spectrum analysis. It uses precomputed twiddle factors for efficiency.
type DFT struct {
	cosTable []float64
	sinTable []float64
}

// NewDFT creates a new DFT instance with precomputed twiddle tables.
func NewDFT() *DFT {
	cosTable := make([]float64, DFTBands*DFTInputSize)
	sinTable := make([]float64, DFTBands*DFTInputSize)

	for k := 0; k < DFTBands; k++ {
		for n := 0; n < DFTInputSize; n++ {
			angle := 2 * math.Pi * float64(k) * float64(n) / float64(DFTInputSize)
			cosTable[k*DFTInputSize+n] = math.Cos(angle)
			sinTable[k*DFTInputSize+n] = math.Sin(angle)
		}
	}

	return &DFT{
		cosTable: cosTable,
		sinTable: sinTable,
	}
}

// Compute applies the DFT to the given samples and returns 64 log-normalized
// magnitude values in the range [0, 1].
// The input samples should be 1024 elements; if fewer are provided, the function
// returns zeros.
func (d *DFT) Compute(samples []float32) []float64 {
	output := make([]float64, DFTBands)

	// If we have fewer samples than DFTInputSize, pad with zeros
	// But per spec: short input returns 64 zeros without panic
	if len(samples) < DFTInputSize {
		return output
	}

	// Apply DFT for each of the 64 frequency bins
	for k := 0; k < DFTBands; k++ {
		var real, imag float64

		offset := k * DFTInputSize
		for n := 0; n < DFTInputSize; n++ {
			sample := float64(samples[n])
			real += sample * d.cosTable[offset+n]
			imag += sample * d.sinTable[offset+n]
		}

		// Compute magnitude
		magnitude := math.Sqrt(real*real + imag*imag)

		// Normalize: mag / N, then apply log normalization
		// log10(1 + 9 * m) gives us values in range [0, 1]
		normalized := magnitude / float64(DFTInputSize)
		output[k] = math.Log10(1 + 9*normalized)
	}

	return output
}
