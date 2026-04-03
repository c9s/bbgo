package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestTradeRingBuffer_Add(t *testing.T) {
	capacity := 3
	b := NewTradeRingBuffer(capacity)

	tr1 := Trade{ID: 1, Time: Time(time.Now()), Quantity: fixedpoint.NewFromFloat(1.0)}
	tr2 := Trade{ID: 2, Time: Time(time.Now()), Quantity: fixedpoint.NewFromFloat(2.0)}
	tr3 := Trade{ID: 3, Time: Time(time.Now()), Quantity: fixedpoint.NewFromFloat(3.0)}
	tr4 := Trade{ID: 4, Time: Time(time.Now()), Quantity: fixedpoint.NewFromFloat(4.0)}

	// Add first trade
	b.Add(tr1)
	assert.Equal(t, 1, b.Count)
	assert.Equal(t, 0, b.Start)
	assert.Equal(t, tr1.ID, b.Trades[0].ID)

	// Add second trade
	b.Add(tr2)
	assert.Equal(t, 2, b.Count)
	assert.Equal(t, 0, b.Start)
	assert.Equal(t, tr2.ID, b.Trades[1].ID)

	// Add third trade (full)
	b.Add(tr3)
	assert.Equal(t, 3, b.Count)
	assert.Equal(t, 0, b.Start)
	assert.Equal(t, tr3.ID, b.Trades[2].ID)

	// Add fourth trade (overwrite tr1)
	b.Add(tr4)
	assert.Equal(t, 3, b.Count)
	assert.Equal(t, 1, b.Start)
	assert.Equal(t, tr4.ID, b.Trades[0].ID)
}

func TestTradeRingBuffer_Filter(t *testing.T) {
	now := time.Now()
	capacity := 5
	b := NewTradeRingBuffer(capacity)

	// trades with sequential timestamps
	tr1 := Trade{ID: 1, Time: Time(now.Add(-40 * time.Second))}
	tr2 := Trade{ID: 2, Time: Time(now.Add(-30 * time.Second))}
	tr3 := Trade{ID: 3, Time: Time(now.Add(-20 * time.Second))}
	tr4 := Trade{ID: 4, Time: Time(now.Add(-10 * time.Second))}
	tr5 := Trade{ID: 5, Time: Time(now)}

	b.Add(tr1)
	b.Add(tr2)
	b.Add(tr3)
	b.Add(tr4)
	b.Add(tr5)

	t.Run("filter none", func(t *testing.T) {
		res := b.Filter(now.Add(-50 * time.Second))
		assert.Equal(t, 5, len(res))
		assert.Equal(t, 5, b.Count)
		assert.Equal(t, 0, b.Start)
	})

	t.Run("filter some", func(t *testing.T) {
		res := b.Filter(now.Add(-25 * time.Second))
		assert.Equal(t, 3, len(res))
		assert.Equal(t, uint64(3), res[0].ID)
		assert.Equal(t, uint64(4), res[1].ID)
		assert.Equal(t, uint64(5), res[2].ID)
		assert.Equal(t, 3, b.Count)
		assert.Equal(t, 2, b.Start)
	})

	t.Run("filter all but last", func(t *testing.T) {
		res := b.Filter(now.Add(-5 * time.Second))
		assert.Equal(t, 1, len(res))
		assert.Equal(t, uint64(5), res[0].ID)
		assert.Equal(t, 1, b.Count)
		assert.Equal(t, 4, b.Start)
	})

	t.Run("filter all", func(t *testing.T) {
		res := b.Filter(now.Add(10 * time.Second))
		assert.Equal(t, 0, len(res))
		assert.Equal(t, 0, b.Count)
	})
}

func TestTradeRingBuffer_Filter_WrapAround(t *testing.T) {
	now := time.Now()
	capacity := 3
	b := NewTradeRingBuffer(capacity)

	// Add 4 trades to cause wrap around
	tr1 := Trade{ID: 1, Time: Time(now.Add(-40 * time.Second))}
	tr2 := Trade{ID: 2, Time: Time(now.Add(-30 * time.Second))}
	tr3 := Trade{ID: 3, Time: Time(now.Add(-20 * time.Second))}
	tr4 := Trade{ID: 4, Time: Time(now.Add(-10 * time.Second))}

	b.Add(tr1) // idx 0
	b.Add(tr2) // idx 1
	b.Add(tr3) // idx 2
	b.Add(tr4) // overwrite tr1 at idx 0, Start = 1

	assert.Equal(t, 3, b.Count)
	assert.Equal(t, 1, b.Start)
	// trades in buffer: tr2 (idx 1), tr3 (idx 2), tr4 (idx 0)

	t.Run("filter middle", func(t *testing.T) {
		// Cutoff between tr2 and tr3
		res := b.Filter(now.Add(-25 * time.Second))
		assert.Equal(t, 2, len(res))
		assert.Equal(t, uint64(3), res[0].ID)
		assert.Equal(t, uint64(4), res[1].ID)
		assert.Equal(t, 2, b.Start)
		assert.Equal(t, 2, b.Count)
	})
}
