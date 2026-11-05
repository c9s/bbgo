package supertrend

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

// The expected values below were captured from the legacy indicator.Supertrend, the legacy strategy-local LinReg
// and the legacy indicator.DEMA on the same synthetic klines before they were migrated to v2 streams.

func TestSupertrendStream_LegacyCompatible(t *testing.T) {
	kLines := syntheticKLines(80, 7)

	source := &indicatorv2.KLineStream{}
	st := NewSupertrendStream(source, 5, 1.5)

	var signals, directions []types.Direction
	source.OnUpdate(func(k types.KLine) {
		signals = append(signals, st.GetSignal())
		directions = append(directions, st.Direction())
	})
	source.BackFill(kLines)

	expectedSignals := []types.Direction{
		0, 0, -1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, -1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -1,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
	expectedDirections := []types.Direction{
		1, 1, -1, -1, -1, -1, -1, -1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, -1, -1, -1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, -1,
		-1, -1, -1, -1, -1, -1, -1, -1, -1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	}
	assert.Equal(t, expectedSignals, signals)
	assert.Equal(t, expectedDirections, directions)

	assert.Equal(t, 80, st.Length())
	at := func(s types.Series, i int) float64 { return s.Last(s.Length() - 1 - i) }

	const delta = 1e-6
	for i, v := range map[int]float64{2: 103.94, 3: 103.68655737704918, 8: 98.97165600804702, 29: 118.08998094503228, 32: 113.82220395029839, 59: 131.46646885107648, 79: 130.05555119069874} {
		assert.InDelta(t, v, at(st, i), delta, "trend price #%d", i)
	}
	for i, v := range map[int]float64{2: 100.84, 3: 96.6434426229508, 8: 98.97165600804702, 29: 113.35214324669944, 32: 113.82220395029839, 59: 125.44489451573256, 79: 130.05555119069874} {
		assert.InDelta(t, v, at(st.SupportLine, i), delta, "support #%d", i)
	}
	for i, v := range map[int]float64{2: 103.94, 3: 103.68655737704918, 8: 103.08878048780488, 29: 118.08998094503228, 32: 117.72413845557932, 59: 131.46646885107648, 79: 138.6135590401772} {
		assert.InDelta(t, v, at(st.ResistanceLine, i), delta, "resistance #%d", i)
	}

	// the legacy ATR skips the first kline, so the v2 ATR series is shifted by 1
	assert.Equal(t, 79, st.AverageTrueRange.Length())
	for i, v := range map[int]float64{2: 2.3044444444444423, 3: 2.34770491803279, 8: 2.5422293279686485, 29: 2.5766539633548575, 59: 2.867645900717648, 79: 2.9525648233666466} {
		assert.InDelta(t, v, at(st.AverageTrueRange, i-1), delta, "atr #%d", i)
	}
	assert.InDelta(t, 130.05555119069874, st.LastSupertrendSupport(), delta)
	assert.InDelta(t, 138.6135590401772, st.LastSupertrendResistance(), delta)
}

func TestLinReg_LegacyCompatible(t *testing.T) {
	kLines := syntheticKLines(80, 7)

	source := &indicatorv2.KLineStream{}
	lr := &LinReg{IntervalWindow: types.IntervalWindow{Window: 6, Interval: types.Interval1m}}
	assert.Equal(t, types.Direction(types.DirectionNone), lr.GetSignal())

	lr.Bind(indicatorv2.ClosePrices(source))
	source.BackFill(kLines)

	assert.Equal(t, 80, lr.Length())
	at := func(i int) float64 { return lr.Last(lr.Length() - 1 - i) }
	for i, v := range map[int]float64{0: 0, 2: 0, 4: 0, 5: -0.26857142857143684, 8: 0.804000000000002, 29: -0.09999999999999432, 32: 0.22314285714286086, 59: 0.13857142857143573, 79: 0.2148571434285941} {
		assert.InDelta(t, v, at(i), 1e-6, "slope #%d", i)
	}
	assert.Equal(t, types.Direction(types.DirectionUp), lr.GetSignal())
}

func TestDoubleDema_LegacyCompatible(t *testing.T) {
	kLines := syntheticKLines(80, 7)

	closePrices := types.NewFloat64Series()
	dd := &DoubleDema{
		fastDEMA: indicatorv2.DEMA(closePrices, 4),
		slowDEMA: indicatorv2.DEMA(closePrices, 9),
	}

	var signals []types.Direction
	for _, k := range kLines {
		closePrices.PushAndEmit(k.Close.Float64())
		signals = append(signals, dd.getDemaSignal(k.Open.Float64(), k.Close.Float64()))
	}

	expected := []types.Direction{
		0, 1, -1, -1, 0, 1, 0, 1, 1, -1, 1, -1, 1, -1, 0, 1, 1, 0, 0, 1,
		1, 1, 1, 0, 1, 1, 0, 0, -1, -1, 0, 1, 1, 0, -1, 0, -1, 1, -1, 1,
		-1, 0, -1, 0, -1, 0, 1, 1, 0, 1, 1, 1, 0, 0, 0, 0, 1, 1, -1, -1,
		0, -1, 0, -1, 0, -1, 1, 0, 1, 1, -1, 1, 0, -1, 1, 1, 1, -1, 0, -1,
	}
	assert.Equal(t, expected, signals)

	at := func(i int) float64 { return dd.fastDEMA.Last(dd.fastDEMA.Length() - 1 - i) }
	for i, v := range map[int]float64{2: 101.16799999999999, 3: 99.78687999999998, 8: 102.9569276928, 29: 114.76776054391122, 32: 117.43227544828709, 59: 126.79907050946711, 79: 134.31538881115532} {
		assert.InDelta(t, v, at(i), 1e-6, "fast DEMA #%d", i)
	}
}
