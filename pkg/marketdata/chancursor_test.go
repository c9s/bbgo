package marketdata_test

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/marketdata"
)

func tradeEvent(ms int64) *marketdata.Event {
	return &marketdata.Event{
		Type:   marketdata.EventTypeTrade,
		Symbol: "BTCUSDT",
		Key:    marketdata.OrderKey{TimeNs: ms * 1e6, Rank: marketdata.RankTrade},
	}
}

func TestChanCursor_ProducerConsumer(t *testing.T) {
	c := marketdata.NewChanCursor(context.Background(), 4)

	go func() {
		for i := int64(1); i <= 3; i++ {
			if !c.Push(tradeEvent(i * 1000)) {
				return
			}
		}
		c.Finish()
	}()

	var times []int64
	for c.Next() {
		times = append(times, c.Event().Key.TimeNs/1e6)
	}

	require.NoError(t, c.Err())
	assert.Equal(t, []int64{1000, 2000, 3000}, times)
	require.NoError(t, c.Close())
}

func TestChanCursor_FailSurfacesViaErr(t *testing.T) {
	boom := errors.New("upstream died")
	c := marketdata.NewChanCursor(context.Background(), 4)

	go func() {
		c.Push(tradeEvent(1000))
		c.Fail(boom)
	}()

	var count int
	for c.Next() {
		count++
	}

	assert.Equal(t, 1, count)
	assert.ErrorIs(t, c.Err(), boom)
	require.NoError(t, c.Close())
}

// TestChanCursor_BlocksWhenFull verifies that backpressure is real: with
// OverflowBlock the producer stalls rather than dropping events.
// TestChanCursor_DrainsBufferedEventsBeforeFailing is a regression test: Next
// used to check the error before reading the channel, so events the producer had
// already delivered were thrown away when a failure arrived. That silently
// truncated the stream right before the error explaining it.
func TestChanCursor_DrainsBufferedEventsBeforeFailing(t *testing.T) {
	boom := errors.New("upstream died")
	c := marketdata.NewChanCursor(context.Background(), 16)

	// Deliver three events and then fail, with nothing consumed in between.
	for i := int64(1); i <= 3; i++ {
		require.True(t, c.Push(tradeEvent(i*1000)))
	}
	c.Fail(boom)

	var times []int64
	for c.Next() {
		times = append(times, c.Event().Key.TimeNs/1e6)
	}

	assert.Equal(t, []int64{1000, 2000, 3000}, times,
		"events that arrived successfully must still be delivered")
	assert.ErrorIs(t, c.Err(), boom, "and the error must follow them")
	require.NoError(t, c.Close())
}

// TestChanCursor_DrainsBufferedEventsBeforeCancel is the same property for
// cancellation.
func TestChanCursor_DrainsBufferedEventsBeforeCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := marketdata.NewChanCursor(ctx, 16)

	require.True(t, c.Push(tradeEvent(1000)))
	require.True(t, c.Push(tradeEvent(2000)))
	cancel()

	var count int
	for c.Next() {
		count++
	}

	assert.Equal(t, 2, count, "buffered events must survive a cancellation")
	assert.ErrorIs(t, c.Err(), context.Canceled)
	require.NoError(t, c.Close())
}

func TestChanCursor_BlocksWhenFull(t *testing.T) {
	c := marketdata.NewChanCursor(context.Background(), 2)

	var pushed atomic.Int64
	go func() {
		for i := int64(1); i <= 10; i++ {
			if !c.Push(tradeEvent(i * 1000)) {
				return
			}
			pushed.Add(1)
		}
		c.Finish()
	}()

	// Give the producer a chance to fill the buffer and block. It can get at
	// most buffer+1 in before Push blocks on the channel send.
	assert.Eventually(t, func() bool { return pushed.Load() >= 2 }, time.Second, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	assert.LessOrEqual(t, pushed.Load(), int64(3),
		"producer must block once the buffer is full instead of dropping events")

	var count int
	for c.Next() {
		count++
	}
	assert.Equal(t, 10, count, "no event may be dropped")
	require.NoError(t, c.Close())
}

func TestChanCursor_OverflowFail(t *testing.T) {
	c := marketdata.NewChanCursor(context.Background(), 1,
		marketdata.WithOverflowPolicy(marketdata.OverflowFail))

	// Nothing is consuming, so the second push overflows.
	assert.True(t, c.Push(tradeEvent(1000)))
	assert.False(t, c.Push(tradeEvent(2000)))

	for c.Next() {
	}
	assert.ErrorIs(t, c.Err(), marketdata.ErrOverflow)
	require.NoError(t, c.Close())
}

// TestChanCursor_CloseUnblocksProducer is the goroutine-leak guard: a consumer
// that stops early must not strand the producer.
func TestChanCursor_CloseUnblocksProducer(t *testing.T) {
	before := runtime.NumGoroutine()

	c := marketdata.NewChanCursor(context.Background(), 1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := int64(1); i <= 1000; i++ {
			if !c.Push(tradeEvent(i * 1000)) {
				return
			}
		}
		c.Finish()
	}()

	require.True(t, c.Next())
	require.NoError(t, c.Close())

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("producer did not return after Close; it is leaked")
	}

	assert.Eventually(t, func() bool { return runtime.NumGoroutine() <= before+1 },
		time.Second, 10*time.Millisecond, "goroutines leaked")
}

func TestChanCursor_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := marketdata.NewChanCursor(ctx, 4)

	cancel()

	assert.False(t, c.Next())
	assert.ErrorIs(t, c.Err(), context.Canceled)
	require.NoError(t, c.Close())
}

func TestChanCursor_DoubleCloseIsSafe(t *testing.T) {
	c := marketdata.NewChanCursor(context.Background(), 1)
	require.NoError(t, c.Close())
	require.NoError(t, c.Close())
	assert.False(t, c.Push(tradeEvent(1000)), "Push after Close must report false")
}
