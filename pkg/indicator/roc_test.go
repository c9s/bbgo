package indicator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestROC_Update(t *testing.T) {
	roc := &ROC{
		IntervalWindow: types.IntervalWindow{Window: 3},
	}

	prices := []float64{10.0, 12.0, 11.0, 15.0, 18.0, 14.0}
	for i, p := range prices {
		roc.PushK(types.KLine{
			Close:   fixedpoint.NewFromFloat(p),
			EndTime: types.Time(time.Now().Add(time.Duration(i) * time.Minute)),
		})
	}

	assert.Equal(t, 3, roc.Length())
	assert.InDelta(t, 50.0, roc.Index(2), 0.0001)
	assert.InDelta(t, 50.0, roc.Index(1), 0.0001)
	assert.InDelta(t, 27.2727, roc.Index(0), 0.001)
	assert.InDelta(t, 27.2727, roc.Last(0), 0.001)
}

func TestROC_ZeroDenominator(t *testing.T) {
	roc := &ROC{
		IntervalWindow: types.IntervalWindow{Window: 1},
	}

	roc.PushK(types.KLine{
		Close:   fixedpoint.NewFromFloat(0),
		EndTime: types.Time(time.Now()),
	})
	roc.PushK(types.KLine{
		Close:   fixedpoint.NewFromFloat(10),
		EndTime: types.Time(time.Now().Add(time.Minute)),
	})

	assert.Equal(t, 1, roc.Length())
	assert.InDelta(t, 0.0, roc.Last(0), 0.0001)
}
