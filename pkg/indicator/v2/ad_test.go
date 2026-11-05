package indicatorv2

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

// randomWalkKLines generates a deterministic random walk of klines, used to
// compare the v2 indicators against the legacy implementations.
func randomWalkKLines(n int, seed int64) []types.KLine {
	r := rand.New(rand.NewSource(seed))
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	price := 100.0
	var kLines []types.KLine
	for i := 0; i < n; i++ {
		open := price
		price += (r.Float64() - 0.5) * 4
		cloze := price
		high := max(open, cloze) + r.Float64()*2
		low := min(open, cloze) - r.Float64()*2
		kLines = append(kLines, types.KLine{
			Open:      fixedpoint.NewFromFloat(open),
			High:      fixedpoint.NewFromFloat(high),
			Low:       fixedpoint.NewFromFloat(low),
			Close:     fixedpoint.NewFromFloat(cloze),
			Volume:    fixedpoint.NewFromFloat(r.Float64() * 1000),
			StartTime: types.Time(start.Add(time.Duration(i) * time.Minute)),
			EndTime:   types.Time(start.Add(time.Duration(i+1)*time.Minute - time.Millisecond)),
		})
	}
	return kLines
}

func Test_AD(t *testing.T) {
	t.Run("hand_computed", func(t *testing.T) {
		kLines := []types.KLine{
			{High: fixedpoint.NewFromFloat(10), Low: fixedpoint.NewFromFloat(8), Close: fixedpoint.NewFromFloat(9.5), Volume: fixedpoint.NewFromFloat(100)},
			{High: fixedpoint.NewFromFloat(11), Low: fixedpoint.NewFromFloat(9), Close: fixedpoint.NewFromFloat(9), Volume: fixedpoint.NewFromFloat(200)},
			{High: fixedpoint.NewFromFloat(10), Low: fixedpoint.NewFromFloat(10), Close: fixedpoint.NewFromFloat(10), Volume: fixedpoint.NewFromFloat(300)},
		}

		stream := &KLineStream{}
		ad := AD(stream)
		stream.BackFill(kLines)

		// MFV1 = ((19 - 18) / 2) * 100 = 50
		// MFV2 = ((18 - 20) / 2) * 200 = -200
		// MFV3 = 0 (high == low)
		assert.Equal(t, 3, ad.Length())
		assert.InDeltaSlice(t, []float64{50, -150, -150}, []float64(ad.Slice), 1e-9)
	})

	t.Run("compare_with_legacy", func(t *testing.T) {
		kLines := randomWalkKLines(200, 1)

		stream := &KLineStream{}
		ad := AD(stream)
		stream.BackFill(kLines)

		legacy := &indicator.AD{}
		legacy.CalculateAndUpdate(kLines)

		assert.Equal(t, legacy.Length(), ad.Length())
		assert.InDeltaSlice(t, []float64(legacy.Values), []float64(ad.Slice), 1e-6)
	})
}
