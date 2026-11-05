package harmonic

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

// The expected values below were captured from the legacy SHARK indicator (bound with BindK/LoadK)
// on the same synthetic klines before it was migrated to a v2 stream. Only the non-zero scores are listed.
func TestSHARK_LegacyCompatible(t *testing.T) {
	testCases := []struct {
		seed                int64
		values, long, short map[int]float64
	}{
		{
			seed:   3,
			values: map[int]float64{57: -7, 58: -4, 60: -5, 61: -1, 74: 12, 77: -1, 85: -1, 89: -1, 90: -1, 174: 2, 197: 12, 198: 1, 199: 12, 200: 12, 201: 12, 202: 7, 203: 7, 204: 12, 205: 11, 206: 1, 208: 13, 209: 7, 211: 7, 212: 9, 213: 1, 217: 1, 219: 1, 220: 1, 221: 1, 222: 1, 225: 1, 226: 1, 227: 1, 228: 1, 230: -4, 235: -1, 236: 1, 237: -1, 239: 4, 240: 2},
			long:   map[int]float64{74: 12, 174: 2, 197: 12, 198: 1, 199: 12, 200: 12, 201: 12, 202: 7, 203: 7, 204: 12, 205: 11, 206: 1, 208: 13, 209: 7, 211: 7, 212: 9, 213: 1, 217: 1, 219: 1, 220: 1, 221: 1, 222: 1, 225: 1, 226: 1, 227: 1, 228: 1, 236: 1, 239: 4, 240: 2},
			short:  map[int]float64{57: 7, 58: 4, 60: 5, 61: 1, 77: 1, 85: 1, 89: 1, 90: 1, 230: 4, 235: 1, 237: 1},
		},
		{
			seed:   11,
			values: map[int]float64{8: 2, 9: 5, 12: 5, 14: 5, 60: 1, 68: 1, 69: 1, 72: -2, 78: 6, 79: 11, 80: 5, 81: 2, 82: 4, 84: 4, 85: 2, 90: 3, 91: 2, 92: 3, 93: 2, 94: 2, 95: 1, 96: 2, 114: 2, 115: 2, 117: 2, 120: 2, 124: 2, 126: 2, 127: 2, 128: 2, 130: 2, 132: 2, 139: 2, 145: 2},
			long:   map[int]float64{8: 2, 9: 5, 12: 5, 14: 5, 60: 1, 68: 1, 69: 1, 78: 6, 79: 11, 80: 5, 81: 2, 82: 4, 84: 4, 85: 2, 90: 3, 91: 2, 92: 3, 93: 2, 94: 2, 95: 1, 96: 2, 114: 2, 115: 2, 117: 2, 120: 2, 124: 2, 126: 2, 127: 2, 128: 2, 130: 2, 132: 2, 139: 2, 145: 2},
			short:  map[int]float64{72: 2},
		},
	}

	// expand the sparse (non-zero) values into a dense slice
	dense := func(n int, m map[int]float64) []float64 {
		out := make([]float64, n)
		for i, v := range m {
			out[i] = v
		}
		return out
	}

	for _, tc := range testCases {
		source := &indicatorv2.KLineStream{}
		inc := NewSHARK(source, 60)

		var updates int
		inc.OnUpdate(func(v float64) { updates++ })
		source.BackFill(syntheticKLines(300, tc.seed))

		// the values are pushed once the window is filled: 300 - 60 + 1
		assert.Equal(t, 241, inc.Length())
		assert.Equal(t, 241, updates)
		assert.Equal(t, dense(241, tc.values), []float64(inc.Slice), "seed %d", tc.seed)
		assert.Equal(t, dense(241, tc.long), []float64(inc.LongScores), "seed %d", tc.seed)
		assert.Equal(t, dense(241, tc.short), []float64(inc.ShortScores), "seed %d", tc.seed)
		assert.Len(t, inc.Array(60), 60)
		assert.Equal(t, inc.Slice[240], inc.Last(0))
	}
}
