package elliottwave

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_ElliottWave(t *testing.T) {
	ewo := NewElliottWave(2, 3)

	ewo.Update(1)
	ewo.Update(2)
	// the slow SMA window is not filled yet
	assert.Equal(t, 0, ewo.Length())
	assert.Len(t, types.Array(ewo, 4), 0)

	ewo.Update(3) // quick = 2.5, slow = 2
	assert.Equal(t, 1, ewo.Length())
	assert.InDelta(t, 2.5/2-1, ewo.Last(0), 1e-12)

	ewo.Update(6) // quick = 4.5, slow = 11/3
	assert.Equal(t, 2, ewo.Length())
	assert.InDelta(t, 4.5/(11./3)-1, ewo.Last(0), 1e-12)
	assert.InDelta(t, 2.5/2-1, ewo.Last(1), 1e-12)
	assert.Equal(t, []float64{ewo.Last(0), ewo.Last(1)}, types.Array(ewo, 4))
}

func Test_klineFeedATR(t *testing.T) {
	feed := &klineFeed{}
	atr := indicatorv2.ATR2(feed, 2)

	now := time.Now()
	k := func(i int, h, l, c float64) types.KLine {
		return types.KLine{
			EndTime: types.Time(now.Add(time.Duration(i) * time.Minute)),
			High:    fixedpoint.NewFromFloat(h),
			Low:     fixedpoint.NewFromFloat(l),
			Close:   fixedpoint.NewFromFloat(c),
		}
	}

	feed.Push(k(0, 11, 9, 10))
	assert.Equal(t, 0, atr.Length())
	feed.Push(k(1, 12, 10, 11)) // tr = 2
	assert.InDelta(t, 2.0, atr.Last(0), 1e-12)
	// a kline that does not end after the last kline is ignored
	feed.Push(k(1, 100, 0, 50))
	assert.Equal(t, 1, atr.Length())
	feed.Push(k(2, 15, 11, 14)) // tr = 4, adjusted rma: 2 + (4-2)/1.5
	assert.InDelta(t, 2+2/1.5, atr.Last(0), 1e-12)
}
