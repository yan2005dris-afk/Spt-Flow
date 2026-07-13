package ui

import (
	"math"
	"testing"
)

func TestVisualizer_Update_Playing(t *testing.T) {
	v := NewVisualizer(10, 5) // width=10, maxHeight=5

	// Initially, heights can be 0 or calculated
	v.Update(true) // playing=true

	// Verify that height for column i is calculated and non-negative
	hasPositive := false
	for i := 0; i < v.Width; i++ {
		h := v.Heights[i]
		if h < 0 || h > 5.0 {
			t.Errorf("Expected height for col %d to be between 0 and 5, got %f", i, h)
		}
		if h > 0 {
			hasPositive = true
		}
	}
	if !hasPositive {
		t.Error("Expected some positive heights while playing")
	}

	// Verify that tick increments and heights change
	h0 := make([]float64, v.Width)
	copy(h0, v.Heights)

	v.Update(true)
	changed := false
	for i := 0; i < v.Width; i++ {
		if v.Heights[i] != h0[i] {
			changed = true
			break
		}
	}
	if !changed {
		t.Error("Expected heights to change on subsequent ticks when playing")
	}
}

func TestVisualizer_Update_Paused_Decay(t *testing.T) {
	v := NewVisualizer(10, 5)
	v.Update(true) // Get some initial heights

	// Capture initial sum of heights
	sum0 := 0.0
	for _, h := range v.Heights {
		sum0 += h
	}

	if sum0 == 0 {
		t.Fatal("Expected non-zero heights to start decay test")
	}

	// Update with playing=false to decay
	v.Update(false)

	sum1 := 0.0
	for _, h := range v.Heights {
		sum1 += h
	}

	// Heights should decay by factor of 0.8
	expectedSum := sum0 * 0.8
	if math.Abs(sum1-expectedSum) > 0.01 {
		t.Errorf("Expected sum to decay to %f, got %f (difference too large)", expectedSum, sum1)
	}

	// Decay further
	v.Update(false)
	sum2 := 0.0
	for _, h := range v.Heights {
		sum2 += h
	}
	expectedSum2 := sum1 * 0.8
	if math.Abs(sum2-expectedSum2) > 0.01 {
		t.Errorf("Expected sum to decay further to %f, got %f", expectedSum2, sum2)
	}
}
