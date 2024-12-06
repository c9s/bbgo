package iforest

import (
	"testing"

	"gonum.org/v1/gonum/mat"
)

func TestSample(t *testing.T) {
	// Prepare sample data
	data := mat.NewDense(10, 2, nil)
	for i := 0; i < 10; i++ {
		data.Set(i, 0, float64(i))
		data.Set(i, 1, float64(i*10))
	}
	// Test sampling with size less than number of samples
	sampleSize := 5
	sampled := sample(data, sampleSize)
	rows, _ := sampled.Dims()
	if rows != sampleSize {
		t.Errorf("Expected %d rows, got %d", sampleSize, rows)
	}
	// Test sampling with size greater than number of samples
	sampleSize = 15
	sampled = sample(data, sampleSize)
	rows, _ = sampled.Dims()
	if rows != 10 {
		t.Errorf("Expected 10 rows, got %d", rows)
	}
}

func TestPerm(t *testing.T) {
	n := 10
	p := perm(n)
	if len(p) != n {
		t.Errorf("Expected permutation length %d, got %d", n, len(p))
	}
	seen := make(map[int]bool)
	for _, v := range p {
		if v < 0 || v >= n {
			t.Errorf("Value out of range: %d", v)
		}
		if seen[v] {
			t.Errorf("Duplicate value: %d", v)
		}
		seen[v] = true
	}
}
