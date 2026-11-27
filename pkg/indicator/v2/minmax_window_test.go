package indicatorv2

import (
	"math/rand"
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

// windowExtremes is the brute force windowed min/max over the whole history.
func windowExtremes(history []float64, window int) (min, max float64) {
	start := len(history) - window
	if start < 0 {
		start = 0
	}

	min, max = history[start], history[start]
	for _, v := range history[start:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	return min, max
}

// TestMINMAX_SlidingWindow checks MIN and MAX against a brute force window over
// a stream long enough to pass the Truncate() threshold of the output series.
// A windowed extreme that is derived from the output slice length silently stops
// sliding once that slice is truncated, which this test is here to catch.
func TestMINMAX_SlidingWindow(t *testing.T) {
	for _, window := range []int{1, 2, 3, 5, 9, 14} {
		source := types.NewFloat64Series()
		minStream := MIN(source, window)
		maxStream := MAX(source, window)

		r := rand.New(rand.NewSource(42))

		var history []float64
		for i := 0; i < MaxSliceSize+TruncateSize; i++ {
			v := float64(r.Intn(100))
			history = append(history, v)
			source.PushAndEmit(v)

			expectedMin, expectedMax := windowExtremes(history, window)
			if got := minStream.Last(0); got != expectedMin {
				t.Fatalf("MIN(window=%d) at %d: got %v, want %v", window, i, got, expectedMin)
			}
			if got := maxStream.Last(0); got != expectedMax {
				t.Fatalf("MAX(window=%d) at %d: got %v, want %v", window, i, got, expectedMax)
			}
		}
	}
}
