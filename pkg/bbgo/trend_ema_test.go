package bbgo

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_TrendEMA(t *testing.T) {
	t.Run("Test Trend EMA", func(t *testing.T) {
		trendEMA_test := TrendEMA{
			IntervalWindow: types.IntervalWindow{Window: 1},
		}
		trendEMA_test.last = 1000.0
		trendEMA_test.current = 1200.0

		if trendEMA_test.Gradient() != 1.2 {
			t.Errorf("Gradient() = %v, want %v", trendEMA_test.Gradient(), 1.2)
		}
	})
}
