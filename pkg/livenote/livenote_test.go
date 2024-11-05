package livenote

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestLiveNotePool(t *testing.T) {
	t.Run("same-kline", func(t *testing.T) {
		pool := NewPool()
		k := &types.KLine{
			Symbol:    "BTCUSDT",
			Interval:  types.Interval1m,
			StartTime: types.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
		}

		note := pool.Update(k)
		note2 := pool.Update(k)
		assert.Equal(t, note, note2, "the returned note object should be the same")
	})

	t.Run("different-kline", func(t *testing.T) {
		pool := NewPool()
		k := &types.KLine{
			Symbol:    "BTCUSDT",
			Interval:  types.Interval1m,
			StartTime: types.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
		}

		k2 := &types.KLine{
			Symbol:    "BTCUSDT",
			Interval:  types.Interval1m,
			StartTime: types.Time(time.Date(2021, 1, 1, 0, 1, 0, 0, time.UTC)),
		}

		note := pool.Update(k)
		note2 := pool.Update(k2)
		assert.NotEqual(t, note, note2, "the returned note object should be different")
	})
}
