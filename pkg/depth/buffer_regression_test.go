package depth

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/types"
)

func TestDepthBuffer_ResetWhileFetchingDoesNotSpawnConcurrentFetchers(t *testing.T) {
	var callCount atomic.Int32
	var activeCount atomic.Int32
	var maxActive atomic.Int32

	firstFetchStarted := make(chan struct{})
	releaseFirstFetch := make(chan struct{})

	buf := NewBuffer(func() (types.SliceOrderBook, int64, error) {
		cur := activeCount.Add(1)
		for {
			prev := maxActive.Load()
			if cur <= prev || maxActive.CompareAndSwap(prev, cur) {
				break
			}
		}
		defer activeCount.Add(-1)

		n := callCount.Add(1)
		if n == 1 {
			close(firstFetchStarted)
			<-releaseFirstFetch
			return types.SliceOrderBook{LastUpdateId: 10}, 10, nil
		}
		return types.SliceOrderBook{LastUpdateId: 20}, 20, nil
	}, time.Millisecond)

	readyC := make(chan types.SliceOrderBook, 4)
	buf.OnReady(func(snapshot types.SliceOrderBook, updates []Update) {
		readyC <- snapshot
	})

	require.NoError(t, buf.AddUpdate(types.SliceOrderBook{LastUpdateId: 11}, 11))

	select {
	case <-firstFetchStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first fetch to start")
	}

	buf.Reset()
	require.NoError(t, buf.AddUpdate(types.SliceOrderBook{LastUpdateId: 21}, 21))

	close(releaseFirstFetch)

	select {
	case snap := <-readyC:
		assert.Equal(t, int64(20), snap.LastUpdateId)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ready callback")
	}

	assert.Equal(t, int32(1), maxActive.Load(), "fetcher must never run concurrently")
	assert.Equal(t, int32(2), callCount.Load(), "stale first fetch must be retried after reset")

	select {
	case extra := <-readyC:
		t.Fatalf("unexpected extra ready callback with snapshot %d", extra.LastUpdateId)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestDepthBuffer_SetSnapshotAfterResetDrainsFetchSignal(t *testing.T) {
	var fetchCalls atomic.Int32
	buf := NewBuffer(func() (types.SliceOrderBook, int64, error) {
		fetchCalls.Add(1)
		return types.SliceOrderBook{LastUpdateId: 50}, 50, nil
	}, 20*time.Millisecond)

	pushed := make(chan Update, 2)
	buf.OnPush(func(u Update) {
		pushed <- u
	})

	require.NoError(t, buf.SetSnapshot(types.SliceOrderBook{LastUpdateId: 10}, 10))
	buf.Reset()
	require.NoError(t, buf.SetSnapshot(types.SliceOrderBook{LastUpdateId: 100}, 100))

	require.NoError(t, buf.AddUpdate(types.SliceOrderBook{LastUpdateId: 101}, 101))

	select {
	case u := <-pushed:
		assert.Equal(t, int64(101), u.FinalUpdateID)
	case <-time.After(time.Second):
		t.Fatal("expected update 101 to be pushed after SetSnapshot")
	}

	assert.Equal(t, int32(0), fetchCalls.Load())
}

func TestDepthBuffer_CallbackCanCallResetWithoutDeadlock(t *testing.T) {
	buf := NewBuffer(func() (types.SliceOrderBook, int64, error) {
		return types.SliceOrderBook{LastUpdateId: 10}, 10, nil
	}, 10*time.Millisecond)

	resetC := make(chan struct{}, 1)
	buf.OnReset(func() {
		select {
		case resetC <- struct{}{}:
		default:
		}
	})

	buf.OnPush(func(u Update) {
		if u.FinalUpdateID == 11 {
			buf.Reset()
		}
	})

	require.NoError(t, buf.SetSnapshot(types.SliceOrderBook{LastUpdateId: 10}, 10))

	done := make(chan error, 1)
	go func() {
		done <- buf.AddUpdate(types.SliceOrderBook{LastUpdateId: 11}, 11)
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("AddUpdate deadlocked when OnPush called Reset")
	}

	select {
	case <-resetC:
	case <-time.After(2 * time.Second):
		t.Fatal("expected OnReset callback to fire")
	}
}
