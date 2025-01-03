package iforest

import (
	"fmt"
	"math"
	"sort"
)

// Quantile computes the q-th quantile (0 <= q <= 1) of a slice of float64 values.
func Quantile(numbers []float64, q float64) float64 {
	if len(numbers) == 0 {
		panic("numbers must not be empty")
	}
	if q < 0 || q > 1 {
		panic(fmt.Sprintf("q must be in [0, 1], got %v", q))
	}

	sortedNumbers := make([]float64, len(numbers))
	copy(sortedNumbers, numbers)
	sort.Float64s(sortedNumbers)

	n := float64(len(sortedNumbers))
	pos := q * (n - 1)
	lowerIndex := int(math.Floor(pos))
	upperIndex := int(math.Ceil(pos))
	if lowerIndex == upperIndex {
		return sortedNumbers[lowerIndex]
	}

	// linear interpolation
	fraction := pos - float64(lowerIndex)
	return sortedNumbers[lowerIndex] + fraction*(sortedNumbers[upperIndex]-sortedNumbers[lowerIndex])
}
