package drift

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// Test_DriftMA checks DriftMA against a hand-rolled EWMA -> volume weighted drift -> fisher transform pipeline
// (the legacy DriftMA computation).
func Test_DriftMA(t *testing.T) {
	const driftWindow, smootherWindow, fisherWindow = 3, 2, 3

	values := []float64{100, 101, 103, 102, 99, 98, 100, 104, 107, 106, 103, 101}
	weights := []float64{1, 2, 1, 3, 2, 1, 1, 2, 4, 1, 2, 3}

	d := NewDriftMA(driftWindow, smootherWindow, fisherWindow)

	var ema, chng, weightQ, drifts, expected []float64
	var lastValue float64
	m := 2.0 / float64(1+smootherWindow)
	for i, v := range values {
		w := weights[i]
		if len(ema) == 0 {
			ema = append(ema, v)
		} else {
			ema = append(ema, (1-m)*ema[len(ema)-1]+m*v)
		}
		e := ema[len(ema)-1]

		weightQ = append(weightQ, w)
		if len(weightQ) > driftWindow {
			weightQ = weightQ[len(weightQ)-driftWindow:]
		}

		if i == 0 {
			lastValue = e
		} else {
			base := math.Inf(1)
			for _, x := range weightQ {
				base = math.Min(base, x)
			}
			c := math.Log(e/lastValue) / w * base
			lastValue = e
			for j := 0; j < int(w/base); j++ {
				chng = append(chng, c)
			}

			if len(chng) >= driftWindow {
				win := chng[len(chng)-driftWindow:]
				mean := 0.
				for _, x := range win {
					mean += x
				}
				mean /= driftWindow
				variance := 0.
				for _, x := range win {
					variance += (x - mean) * (x - mean)
				}
				variance /= driftWindow
				drift := mean - variance*0.5
				drifts = append(drifts, drift)

				fw := drifts
				if len(fw) > fisherWindow {
					fw = fw[len(fw)-fisherWindow:]
				}
				hi, lo := math.Inf(-1), math.Inf(1)
				for _, x := range fw {
					hi = math.Max(hi, x)
					lo = math.Min(lo, x)
				}
				if hi == lo {
					expected = append(expected, 0)
				} else {
					x := 2*((drift-lo)/(hi-lo)) - 1
					if x == 1 {
						x = 0.9999
					} else if x == -1 {
						x = -0.9999
					}
					expected = append(expected, 0.5*math.Log((1+x)/(1-x)))
				}
			}
		}

		d.Update(v, w)

		if assert.Equal(t, len(expected), d.Length(), "length at %d", i) && len(expected) > 0 {
			assert.InDelta(t, drifts[len(drifts)-1], d.drift.Last(0), 1e-9, "drift at %d", i)
			assert.InDelta(t, expected[len(expected)-1], d.Last(0), 1e-9, "fisher at %d", i)
		}
	}

	assert.Greater(t, d.Length(), 2)
}

func Test_IndicatorStreams(t *testing.T) {
	s := &Strategy{}
	s.HLRangeWindow = 2
	s.ATRWindow = 2
	s.TrendWindow = 2
	s.Window = 2
	s.SmootherWindow = 2
	s.FisherTransformWindow = 2
	s.initIndicatorStreams()

	k := func(h, l, c float64) types.KLine {
		return types.KLine{High: fixedpoint.NewFromFloat(h), Low: fixedpoint.NewFromFloat(l), Close: fixedpoint.NewFromFloat(c)}
	}

	// ATR: the first kline only sets the previous close
	s.kLineFeed.Push(k(11, 9, 10))
	assert.Equal(t, 0, s.atr.Length())
	s.kLineFeed.Push(k(12, 10, 11)) // tr = 2
	assert.InDelta(t, 2.0, s.atr.Last(0), 1e-12)
	s.kLineFeed.Push(k(15, 11, 14)) // tr = 4, adjusted rma: sum = 1.5, 2 + (4-2)/1.5
	assert.InDelta(t, 2+2/1.5, s.atr.Last(0), 1e-12)

	// ma warm-up guard: 0 until the window is filled
	pushFloat64(s.sourceSeries, 10)
	assert.Equal(t, 0.0, s.maLast())
	pushFloat64(s.sourceSeries, 12)
	assert.Equal(t, 11.0, s.maLast())
	// trendLine is fed by the same source: ema(2) = 10 -> 10/3 + 2*12/3
	assert.InDelta(t, 10./3+24./3, s.trendLine.Last(0), 1e-12)
}
