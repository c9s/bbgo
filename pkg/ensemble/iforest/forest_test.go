package iforest

import (
	"testing"
)

func TestIsolationForest(t *testing.T) {
	tests := []struct {
		features    [][]float64
		predictions []int
	}{
		{
			[][]float64{
				{0, 0, 0},
				{0, 0, 0},
				{0, 0, 0},
				{1, 1, 1},
			},
			[]int{0, 0, 0, 1},
		},
	}

	for _, tt := range tests {
		forest := New()
		forest.Fit(tt.features)

		preds := forest.Predict(tt.features)
		for i, pred := range preds {
			if pred != tt.predictions[i] {
				t.Errorf("expected %v, got %v", tt.predictions[i], pred)
			}
		}
	}
}
