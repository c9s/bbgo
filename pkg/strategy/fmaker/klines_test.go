package fmaker

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// syntheticKLines generates n deterministic klines for the factor tests.
func syntheticKLines(n int) []types.KLine {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	var kLines []types.KLine
	for i := 0; i < n; i++ {
		x := float64(i)
		open := 100 + 10*math.Sin(x/7) + 3*math.Cos(x/3)
		closePrice := open + 2*math.Sin(x/2+1)
		high := math.Max(open, closePrice) + 1 + math.Abs(math.Sin(x/5))
		low := math.Min(open, closePrice) - 1 - math.Abs(math.Cos(x/4))
		volume := 1000 + 300*math.Sin(x/9) + 50*math.Cos(x)
		st := start.Add(time.Duration(i) * time.Minute)
		kLines = append(kLines, types.KLine{
			Symbol:    "BTCUSDT",
			Interval:  types.Interval1m,
			StartTime: types.Time(st),
			EndTime:   types.Time(st.Add(time.Minute - time.Millisecond)),
			Open:      fixedpoint.NewFromFloat(open),
			Close:     fixedpoint.NewFromFloat(closePrice),
			High:      fixedpoint.NewFromFloat(high),
			Low:       fixedpoint.NewFromFloat(low),
			Volume:    fixedpoint.NewFromFloat(volume),
			Closed:    true,
		})
	}
	return kLines
}

// summary is a compact fingerprint of a value series used to compare against golden values.
type summary struct {
	Len   int
	First float64
	Last  float64
	Sum   float64
}

func summarize(vs []float64) summary {
	sum := 0.0
	for _, v := range vs {
		sum += v
	}
	return summary{Len: len(vs), First: vs[0], Last: vs[len(vs)-1], Sum: sum}
}

// assertSummary compares the summaries within a tolerance, because the captured values are not reproducible to the
// last digit: the float64 results depend on the architecture (arm64 fuses multiply-adds, amd64 does not) and on the
// fixedpoint implementation (dnum).
func assertSummary(t *testing.T, expected, actual summary, msgAndArgs ...any) {
	t.Helper()

	assert.Equal(t, expected.Len, actual.Len, msgAndArgs...)

	for _, v := range [][2]float64{
		{expected.First, actual.First},
		{expected.Last, actual.Last},
		{expected.Sum, actual.Sum},
	} {
		assert.InDelta(t, v[0], v[1], 1e-6*math.Max(1, math.Abs(v[0])), msgAndArgs...)
	}
}
