package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

/*
import pandas as pd
import pandas_ta as ta
high = pd.Series([1.1, 1.3, 1.5, 1.7, 1.9, 2.2, 2.4, 2.1, 1.8, 1.7])
low = pd.Series([0.9, 1.1, 1.2, 1.5, 1.7, 2.0, 2.2, 1.9, 1.6, 1.5])
close = pd.Series([1.0, 1.2, 1.4, 1.6, 1.8, 2.1, 2.3, 2.0, 1.7, 1.6])
vol = pd.Series([300., 200., 200., 150., 150., 200., 200., 150., 300., 350.])
# kvo = ta.kvo(high, low, close, vol, fast=3, slow=5, signal=1)
# print(kvo)
# # The implementation of kvo in pandas_ta is different from the one defined in investopedia
# # VF is not simply multipying trend
# # Also the value is not multiplied by 100 in pandas_ta
*/
func Test_KlingerOscillator(t *testing.T) {
	high := []float64{1.1, 1.3, 1.5, 1.7, 1.9, 2.2, 2.4, 2.1, 1.8, 1.7}
	low := []float64{0.9, 1.1, 1.2, 1.5, 1.7, 2.0, 2.2, 1.9, 1.6, 1.5}
	cloze := []float64{1.0, 1.2, 1.4, 1.6, 1.8, 2.1, 2.3, 2.0, 1.7, 1.6}
	vol := []float64{300.0, 200.0, 200.0, 150.0, 150.0, 200.0, 200.0, 150.0, 300.0, 350.0}
	expectedVF := []float64{300.0, 0.0, -28.5, -83, -95, -138, -146.7, 0, 100, 175}

	var kLines []types.KLine
	for i := range high {
		kLines = append(kLines, types.KLine{
			High:   fixedpoint.NewFromFloat(high[i]),
			Low:    fixedpoint.NewFromFloat(low[i]),
			Close:  fixedpoint.NewFromFloat(cloze[i]),
			Volume: fixedpoint.NewFromFloat(vol[i]),
		})
	}

	stream := &KLineStream{}
	ko := KlingerOscillator(stream, 3, 5)
	stream.BackFill(kLines)

	const delta = 0.5
	assert.Equal(t, len(expectedVF), ko.VolumeForce.Length())
	for i, want := range expectedVF {
		assert.InDelta(t, want, ko.VolumeForce.Slice[i], delta, "volume force at %d", i)
	}

	// the first kline only initializes the volume force
	assert.Equal(t, len(kLines)-1, ko.Length())
	assert.Equal(t, len(kLines)-1, ko.Fast.Length())
	assert.Equal(t, len(kLines)-1, ko.Slow.Length())
	for i := 0; i < ko.Length(); i++ {
		assert.InDelta(t, ko.Fast.Slice[i]-ko.Slow.Slice[i], ko.Slice[i], 1e-9)
	}
}

func Test_KlingerOscillator_Legacy(t *testing.T) {
	stream := &KLineStream{}
	ko := KlingerOscillator(stream, 0, 0)
	legacy := &indicator.KlingerOscillator{}

	for i := 0; i < 200; i++ {
		c := 100 + float64(i%10) + float64(i%7)*0.3
		k := types.KLine{
			High:   fixedpoint.NewFromFloat(c + 1 + float64(i%3)*0.2),
			Low:    fixedpoint.NewFromFloat(c - 1 - float64(i%4)*0.1),
			Close:  fixedpoint.NewFromFloat(c),
			Volume: fixedpoint.NewFromFloat(100 + float64(i%5)*10),
		}
		stream.EmitUpdate(k)
		legacy.PushK(k)
		assert.Equal(t, legacy.Length(), ko.Length())
		assert.InDelta(t, legacy.VF.Value, ko.VolumeForce.Last(0), 1e-9)
		if legacy.Length() > 0 {
			assert.InDelta(t, legacy.Last(0), ko.Last(0), 1e-6)
		}
	}
}
