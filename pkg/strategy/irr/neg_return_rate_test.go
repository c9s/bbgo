package irr

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// syntheticKLines generates deterministic random walk klines, the prices are rounded to 2 decimals.
func syntheticKLines(n int, seed int64) []types.KLine {
	r := rand.New(rand.NewSource(seed))
	round := func(x float64) float64 { return math.Round(x*100) / 100 }
	price := 100.0
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	var kLines []types.KLine
	for i := 0; i < n; i++ {
		open := round(price)
		price = price * (1 + (r.Float64()-0.5)*0.04)
		cls := round(price)
		high := round(math.Max(open, cls) * (1 + r.Float64()*0.01))
		low := round(math.Min(open, cls) * (1 - r.Float64()*0.01))
		vol := round(1 + r.Float64()*10)
		kLines = append(kLines, types.KLine{
			Symbol:    "BTCUSDT",
			Interval:  types.Interval1m,
			StartTime: types.Time(t),
			EndTime:   types.Time(t.Add(time.Minute - time.Millisecond)),
			Open:      fixedpoint.NewFromFloat(open),
			Close:     fixedpoint.NewFromFloat(cls),
			High:      fixedpoint.NewFromFloat(high),
			Low:       fixedpoint.NewFromFloat(low),
			Volume:    fixedpoint.NewFromFloat(vol),
			Closed:    true,
		})
		t = t.Add(time.Minute)
	}
	return kLines
}

// The expected values below were captured from the legacy NRR indicator (bound with BindK/LoadK)
// on the same synthetic klines before it was migrated to a v2 stream.
func TestNRR_LegacyCompatible(t *testing.T) {
	kLines := syntheticKLines(40, 5)

	source := &indicatorv2.KLineStream{}
	inc := NewNRR(source, 2, 7, true)

	var updates int
	inc.OnUpdate(func(v float64) { updates++ })
	source.BackFill(kLines)

	assert.Equal(t, 40, updates)
	assert.Equal(t, 40, inc.Length())
	assert.Equal(t, 40, len(inc.RankedValues))
	assert.Equal(t, 40, len(inc.ReturnValues))

	const delta = 1e-12
	for i, v := range map[int]float64{0: -0.012199999999999989, 1: 0.000889152341434496, 2: 0.008009492732127033, 6: -0.019047619047619202, 7: -0.008952287260206582, 20: 0.0025347257426746594, 39: -0.0026390382171830318} {
		assert.InDelta(t, v, inc.Slice[i], delta, "neg return #%d", i)
		assert.InDelta(t, -v, inc.ReturnValues[i], delta, "return #%d", i)
	}

	// the ranks of the negative returns in the ranking window, multiplied by the ranking window (7)
	expectedRanks := []int{1, 2, 3, 2, 2, 6, 1, 2, 6, 2, 5, 2, 5, 4, 7, 7, 5, 2, 5, 3, 2, 5, 2, 2, 3, 1, 3, 4, 3, 7, 2, 6, 2, 5, 5, 5, 3, 2, 7, 3}
	for i, r := range expectedRanks {
		assert.InDelta(t, float64(r)/7.0, inc.RankedValues[i], delta, "ranked #%d", i)
	}
}
