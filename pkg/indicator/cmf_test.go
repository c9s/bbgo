package indicator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestCMF_Calculation(t *testing.T) {
	// Bar 1: H=10, L=5, C=7.5 (mid), V=100 => MFM = 0, MFV = 0
	// Bar 2: H=15, L=5, C=15 (high), V=200 => MFM = 1, MFV = 200
	// Bar 3: H=12, L=8, C=8 (low), V=100  => MFM = -1, MFV = -100
	// Window = 3: sumMFV = 100, sumVol = 400 => CMF = 100 / 400 = 0.25

	cmf := &CMF{
		IntervalWindow: types.IntervalWindow{
			Window: 3,
		},
	}

	cmf.Update(10, 5, 7.5, 100)
	assert.Equal(t, 0, cmf.Length(), "should not have values before window is filled")

	cmf.Update(15, 5, 15, 200)
	assert.Equal(t, 0, cmf.Length(), "should not have values before window is filled")

	cmf.Update(12, 8, 8, 100)
	assert.Equal(t, 1, cmf.Length())
	assert.InDelta(t, 0.25, cmf.Last(0), Delta)
	assert.InDelta(t, 0.25, cmf.Index(0), Delta)

	// Bar 4: H=20, L=10, C=20 (high), V=100 => MFM = 1, MFV = 100
	// Window 3 covers bars 2, 3, 4:
	// sumMFV = 200 + (-100) + 100 = 200
	// sumVol = 200 + 100 + 100 = 400
	// CMF = 200 / 400 = 0.5
	cmf.Update(20, 10, 20, 100)
	assert.Equal(t, 2, cmf.Length())
	assert.InDelta(t, 0.5, cmf.Last(0), Delta)
	assert.InDelta(t, 0.25, cmf.Last(1), Delta)
}

func TestCMF_ZeroRangeAndZeroVolume(t *testing.T) {
	cmf := &CMF{
		IntervalWindow: types.IntervalWindow{
			Window: 2,
		},
	}

	// High == Low
	cmf.Update(10, 10, 10, 100)
	cmf.Update(10, 10, 10, 100)
	assert.Equal(t, 1, cmf.Length())
	assert.InDelta(t, 0.0, cmf.Last(0), Delta)

	// Zero Volume
	cmfZeroVol := &CMF{
		IntervalWindow: types.IntervalWindow{
			Window: 2,
		},
	}
	cmfZeroVol.Update(15, 5, 15, 0)
	cmfZeroVol.Update(15, 5, 15, 0)
	assert.Equal(t, 1, cmfZeroVol.Length())
	assert.InDelta(t, 0.0, cmfZeroVol.Last(0), Delta)
}

func TestCMF_CalculateAndUpdate(t *testing.T) {
	kLines := []types.KLine{
		{
			High:    fixedpoint.NewFromFloat(10),
			Low:     fixedpoint.NewFromFloat(5),
			Close:   fixedpoint.NewFromFloat(7.5),
			Volume:  fixedpoint.NewFromFloat(100),
			EndTime: types.Time(time.Unix(100, 0)),
		},
		{
			High:    fixedpoint.NewFromFloat(15),
			Low:     fixedpoint.NewFromFloat(5),
			Close:   fixedpoint.NewFromFloat(15),
			Volume:  fixedpoint.NewFromFloat(200),
			EndTime: types.Time(time.Unix(200, 0)),
		},
		{
			High:    fixedpoint.NewFromFloat(12),
			Low:     fixedpoint.NewFromFloat(8),
			Close:   fixedpoint.NewFromFloat(8),
			Volume:  fixedpoint.NewFromFloat(100),
			EndTime: types.Time(time.Unix(300, 0)),
		},
	}

	var updatedVal float64
	var updateCount int

	cmf := &CMF{
		IntervalWindow: types.IntervalWindow{
			Window: 3,
		},
	}
	cmf.OnUpdate(func(val float64) {
		updatedVal = val
		updateCount++
	})

	cmf.CalculateAndUpdate(kLines)

	assert.Equal(t, 1, cmf.Length())
	assert.InDelta(t, 0.25, cmf.Last(0), Delta)
	assert.Equal(t, 1, updateCount)
	assert.InDelta(t, 0.25, updatedVal, Delta)
	assert.Equal(t, time.Unix(300, 0), cmf.EndTime)

	// Call again with same kLines should not duplicate
	cmf.CalculateAndUpdate(kLines)
	assert.Equal(t, 1, cmf.Length())
	assert.Equal(t, 1, updateCount)
}

func TestCMF_PushK(t *testing.T) {
	cmf := &CMF{
		IntervalWindow: types.IntervalWindow{
			Window: 2,
		},
	}

	var callbacks []float64
	cmf.OnUpdate(func(v float64) {
		callbacks = append(callbacks, v)
	})

	k1 := types.KLine{
		High:    fixedpoint.NewFromFloat(10),
		Low:     fixedpoint.NewFromFloat(5),
		Close:   fixedpoint.NewFromFloat(10),
		Volume:  fixedpoint.NewFromFloat(100),
		EndTime: types.Time(time.Unix(100, 0)),
	}
	k2 := types.KLine{
		High:    fixedpoint.NewFromFloat(10),
		Low:     fixedpoint.NewFromFloat(5),
		Close:   fixedpoint.NewFromFloat(10),
		Volume:  fixedpoint.NewFromFloat(100),
		EndTime: types.Time(time.Unix(200, 0)),
	}

	cmf.PushK(k1)
	assert.Equal(t, 0, len(callbacks))

	cmf.PushK(k2)
	assert.Equal(t, 1, len(callbacks))
	assert.InDelta(t, 1.0, callbacks[0], Delta)
}
