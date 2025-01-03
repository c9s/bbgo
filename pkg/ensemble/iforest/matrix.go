package iforest

import (
	"math"
	"math/rand"
)

// SampleRows randomly selects 'size' rows from the matrix.
func SampleRows(matrix [][]float64, size int) [][]float64 {
	if size <= 0 {
		panic("size must be greater than 0")
	}

	if len(matrix) <= size {
		return matrix
	}

	perm := rand.Perm(len(matrix))
	sampled := make([][]float64, size)
	for i := 0; i < size; i++ {
		sampled[i] = matrix[perm[i]]
	}
	return sampled
}

// Column returns a slice containing the specified column from the matrix.
func Column(matrix [][]float64, columnIndex int) []float64 {
	column := make([]float64, len(matrix))
	for i, row := range matrix {
		column[i] = row[columnIndex]
	}
	return column
}

// MinMax returns the minimum and maximum values from a slice of float64.
func MinMax(floats []float64) (float64, float64) {
	min, max := math.Inf(1), math.Inf(-1)
	for _, v := range floats {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}
