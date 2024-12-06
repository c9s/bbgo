package iforest

import (
	"math/rand"

	"gonum.org/v1/gonum/mat"
)

func sample(samples *mat.Dense, size int) *mat.Dense {
	numSamples, numFeatures := samples.Dims()

	if size >= numSamples {
		return mat.DenseCopyOf(samples)
	}

	indices := perm(numSamples)[:size]
	o := make([]float64, size*numFeatures)
	for i, idx := range indices {
		copy(o[i*numFeatures:], samples.RawRowView(idx))
	}
	return mat.NewDense(size, numFeatures, o)
}

func perm(n int) []int {
	m := make([]int, n)
	for i := 0; i < n; i++ {
		m[i] = i
	}
	shuffle(m)
	return m
}

func shuffle(slice []int) {
	for i := len(slice) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}
