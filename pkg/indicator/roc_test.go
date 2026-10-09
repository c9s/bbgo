package indicator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_ROC(t *testing.T) {
	roc := &ROC{
		IntervalWindow: types.IntervalWindow{
			Window: 3,
		},
	}

	prices := []float64{10.0, 12.0, 11.0, 15.0, 18.0, 14.0}
	for _, p := range prices {
		roc.Update(p)
	}

	assert.Equal(t, 3, roc.Length())
	// 4th element (15.0) vs 1st (10.0) -> (15 - 10) / 10 * 100 = 50.0%
	assert.InDelta(t, 50.0, roc.Index(2), 0.0001)
	// 5th element (18.0) vs 2nd (12.0) -> (18 - 12) / 12 * 100 = 50.0%
	assert.InDelta(t, 50.0, roc.Index(1), 0.0001)
	// 6th element (14.0) vs 3rd (11.0) -> (14 - 11) / 11 * 100 = 27.2727%
	assert.InDelta(t, 27.2727, roc.Index(0), 0.001)
	assert.InDelta(t, 27.2727, roc.Last(0), 0.001)
}

func Test_ROC_PushK(t *testing.T) {
	roc := &ROC{
		IntervalWindow: types.IntervalWindow{
			Window: 2,
		},
	}

	var updatedValues []float64
	roc.OnUpdate(func(val float64) {
		updatedValues = append(updatedValues, val)
	})

	now := time.Now()
	roc.PushK(types.KLine{Close: fixedpoint.NewFromFloat(100.0), EndTime: types.Time(now)})
	roc.PushK(types.KLine{Close: fixedpoint.NewFromFloat(105.0), EndTime: types.Time(now.Add(time.Minute))})
	roc.PushK(types.KLine{Close: fixedpoint.NewFromFloat(110.0), EndTime: types.Time(now.Add(2 * time.Minute))})

	// (110 - 100) / 100 * 100 = 10.0%
	assert.Equal(t, 1, roc.Length())
	assert.InDelta(t, 10.0, roc.Last(0), 0.0001)
	assert.Len(t, updatedValues, 1)
	assert.InDelta(t, 10.0, updatedValues[0], 0.0001)
}
