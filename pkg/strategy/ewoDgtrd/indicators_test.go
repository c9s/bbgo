package ewoDgtrd

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_sma(t *testing.T) {
	s := newSMA(3)
	s.Update(1)
	s.Update(2)
	// like the legacy SMA, there is no value until the window is filled
	assert.Equal(t, 0, s.Length())
	assert.Equal(t, 0.0, s.Last(0))

	s.Update(3)
	assert.Equal(t, 1, s.Length())
	assert.Equal(t, 2.0, s.Last(0))

	s.Update(7)
	assert.Equal(t, 2, s.Length())
	assert.Equal(t, 4.0, s.Last(0))
	assert.Equal(t, 2.0, s.Last(1))
	assert.Equal(t, 0.0, s.Last(2))

	// the SeriesExtend wrapper reads the filled values only
	se := types.NewSeries(s)
	assert.Equal(t, []float64{4, 2}, types.Array(se.Minus(0), 10))
}

func Test_VWEMA(t *testing.T) {
	v := NewVWEMA(3)
	v.UpdateVal(10, 1)
	assert.Equal(t, 10.0, v.Last(0))
	v.UpdateVal(20, 3)
	// pv = 0.5*10 + 0.5*60 = 35, vol = 0.5*1 + 0.5*3 = 2
	assert.InDelta(t, 17.5, v.Last(0), 1e-12)
	assert.Equal(t, 2, v.Length())
}

// cciStochReference computes CCI(28) (mean absolute deviation) -> stochastic(28, 3) D -> SMA(3)
// with the legacy update flow: the stochastic oscillator is updated on every value, with 0 before the CCI is ready.
func cciStochReference(values []float64) []float64 {
	const window = 28
	var cci, k, d, ma []float64
	var stochInput []float64
	for i := range values {
		if i+1 >= window {
			win := values[i+1-window : i+1]
			mean := 0.
			for _, x := range win {
				mean += x
			}
			mean /= window
			md := 0.
			for _, x := range win {
				md += math.Abs(x - mean)
			}
			md /= window
			if md == 0 {
				cci = append(cci, 0)
			} else {
				cci = append(cci, (values[i]-mean)/(0.015*md))
			}
		}

		c := 0.
		if len(cci) > 0 {
			c = cci[len(cci)-1]
		}
		stochInput = append(stochInput, c)
		win := stochInput
		if len(win) > window {
			win = win[len(win)-window:]
		}
		hi, lo := math.Inf(-1), math.Inf(1)
		for _, x := range win {
			hi = math.Max(hi, x)
			lo = math.Min(lo, x)
		}
		if hi == lo {
			k = append(k, 50)
		} else {
			k = append(k, 100*(c-lo)/(hi-lo))
		}
		kw := k
		if len(kw) > 3 {
			kw = kw[len(kw)-3:]
		}
		dv := 0.
		for _, x := range kw {
			dv += x
		}
		d = append(d, dv/float64(len(kw)))

		if len(d) >= 3 {
			ma = append(ma, (d[len(d)-1]+d[len(d)-2]+d[len(d)-3])/3)
		}
	}
	return ma
}

func Test_CCISTOCH(t *testing.T) {
	var values []float64
	for i := 0; i < 80; i++ {
		values = append(values, 100+10*math.Sin(float64(i)/5)+float64(i%7))
	}

	ccis := NewCCISTOCH("", 70, 30)
	for _, v := range values {
		ccis.Update(v)
	}

	expected := cciStochReference(values)
	if assert.Equal(t, len(expected), ccis.ma.Length()) {
		for i := range expected {
			// the CCI values go through fixedpoint klines into the stochastic oscillator
			assert.InDelta(t, expected[len(expected)-1-i], ccis.ma.Last(i), 1e-4, "index %d", i)
		}
	}

	// before the CCI window is filled, the stochastic oscillator is fed with zeros: K = D = 50
	warm := NewCCISTOCH("", 70, 30)
	for _, v := range values[:5] {
		warm.Update(v)
	}
	assert.Equal(t, 3, warm.ma.Length())
	assert.Equal(t, 50.0, warm.ma.Last(0))
	assert.True(t, warm.BuySignal() == false && warm.SellSignal() == false)
}
