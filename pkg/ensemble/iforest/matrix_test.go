package iforest

import (
	"reflect"
	"testing"
)

func TestSampleRows(t *testing.T) {
	tests := []struct {
		name   string
		matrix [][]float64
		size   int
	}{
		{
			name: "Sample size less than matrix length",
			matrix: [][]float64{
				{1.0, 2.0},
				{3.0, 4.0},
				{5.0, 6.0},
			},
			size: 2,
		},
		{
			name: "Sample size equal to matrix length",
			matrix: [][]float64{
				{1.0, 2.0},
				{3.0, 4.0},
				{5.0, 6.0},
			},
			size: 3,
		},
		{
			name: "Sample size greater than matrix length",
			matrix: [][]float64{
				{1.0, 2.0},
				{3.0, 4.0},
			},
			size: 3,
		},
		{
			name: "Sample size zero",
			matrix: [][]float64{
				{1.0, 2.0},
				{3.0, 4.0},
			},
			size: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if tt.size > 0 {
						t.Errorf("SampleRows() panicked with size %d", tt.size)
					}
				}
			}()

			got := SampleRows(tt.matrix, tt.size)
			if tt.size > 0 && len(tt.matrix) > tt.size {
				if len(got) != tt.size {
					t.Errorf("SampleRows() = %v, want length %d", got, tt.size)
				}
			} else {
				if !reflect.DeepEqual(got, tt.matrix) {
					t.Errorf("SampleRows() = %v, want %v", got, tt.matrix)
				}
			}
		})
	}
}

func TestColumn(t *testing.T) {
	tests := []struct {
		name        string
		matrix      [][]float64
		columnIndex int
		want        []float64
	}{
		{
			name: "Valid column index",
			matrix: [][]float64{
				{1.0, 2.0, 3.0},
				{4.0, 5.0, 6.0},
				{7.0, 8.0, 9.0},
			},
			columnIndex: 1,
			want:        []float64{2.0, 5.0, 8.0},
		},
		{
			name: "First column",
			matrix: [][]float64{
				{1.0, 2.0},
				{3.0, 4.0},
			},
			columnIndex: 0,
			want:        []float64{1.0, 3.0},
		},
		{
			name: "Last column",
			matrix: [][]float64{
				{1.0, 2.0},
				{3.0, 4.0},
			},
			columnIndex: 1,
			want:        []float64{2.0, 4.0},
		},
		{
			name: "Single row matrix",
			matrix: [][]float64{
				{1.0, 2.0, 3.0},
			},
			columnIndex: 2,
			want:        []float64{3.0},
		},
		{
			name: "Single column matrix",
			matrix: [][]float64{
				{1.0},
				{2.0},
				{3.0},
			},
			columnIndex: 0,
			want:        []float64{1.0, 2.0, 3.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Column(tt.matrix, tt.columnIndex)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Column() = %v, want %v", got, tt.want)
			}
		})
	}
}
