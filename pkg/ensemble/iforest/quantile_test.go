package iforest

import (
	"testing"
)

func TestQuantile(t *testing.T) {
	tests := []struct {
		numbers  []float64
		q        float64
		expected float64
	}{
		{[]float64{1, 2}, 0.5, 1.5},
		{[]float64{1, 2, 3, 4, 5}, 0.5, 3},
		{[]float64{1, 2, 3, 4, 5}, 1.0, 5},
		{[]float64{1, 2, 3, 4, 5}, 0.0, 1},
		{[]float64{1, 3, 3, 6, 7, 8, 9}, 0.25, 3},
		{[]float64{1, 3, 3, 6, 7, 8, 9}, 0.75, 7.5},
		{[]float64{1, 2, 3, 4, 5, 6, 8, 9}, 0.5, 4.5},
	}

	for _, tt := range tests {
		actual := Quantile(tt.numbers, tt.q)
		if actual != tt.expected {
			t.Errorf("Quantile(%v, %v) == %v, expected %v", tt.numbers, tt.q, actual, tt.expected)
		}
	}
}

func TestQuantilePanics(t *testing.T) {
	tests := []struct {
		numbers []float64
		q       float64
	}{
		{[]float64{}, 0.5},
		{[]float64{1, 2, 3}, -0.1},
		{[]float64{1, 2, 3}, 1.1},
	}

	for _, tt := range tests {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Quantile(%v, %v) did not panic", tt.numbers, tt.q)
			}
		}()
		Quantile(tt.numbers, tt.q)
	}
}
