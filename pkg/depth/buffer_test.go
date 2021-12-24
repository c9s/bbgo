package depth

import (
	"context"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestDepthBuffer_ReadyState(t *testing.T) {
	buf := NewDepthBuffer("", func() (book types.SliceOrderBook, finalID int64, err error) {
		return types.SliceOrderBook{
			Bids: types.PriceVolumeSlice{
				{Price: 100, Volume: 1},
			},
			Asks: types.PriceVolumeSlice{
				{Price: 99, Volume: 1},
			},
		}, 33, nil
	})

	readyC := make(chan struct{})
	buf.OnReady(func(snapshot types.SliceOrderBook, updates []Update) {
		assert.Greater(t, len(updates), 33)
		close(readyC)
	})

	var updateID int64 = 1
	for ; updateID < 100; updateID++ {
		buf.AddUpdate(
			types.SliceOrderBook{
				Bids: types.PriceVolumeSlice{
					{Price: 100, Volume: fixedpoint.Value(updateID)},
				},
				Asks: types.PriceVolumeSlice{
					{Price: 99, Volume: fixedpoint.Value(updateID)},
				},
			}, updateID)
	}

	<-readyC
}

func TestDepthBuffer_CorruptedUpdateAtTheBeginning(t *testing.T) {
	// snapshot starts from 30,
	// the first ready event should have a snapshot(30) and updates (31~50)
	var snapshotFinalID int64 = 0
	buf := NewDepthBuffer("", func() (types.SliceOrderBook, int64, error) {
		snapshotFinalID += 30
		return types.SliceOrderBook{
			Bids: types.PriceVolumeSlice{
				{Price: 100, Volume: 1},
			},
			Asks: types.PriceVolumeSlice{
				{Price: 99, Volume: 1},
			},
		}, snapshotFinalID, nil
	})

	resetC := make(chan struct{}, 1)

	buf.OnReset(func() {
		resetC <- struct{}{}
	})

	var updateID int64 = 10
	for ; updateID < 100; updateID++ {
		if updateID == 50 {
			updateID += 5
		}

		buf.AddUpdate(types.SliceOrderBook{
			Bids: types.PriceVolumeSlice{
				{Price: 100, Volume: fixedpoint.Value(updateID)},
			},
			Asks: types.PriceVolumeSlice{
				{Price: 99, Volume: fixedpoint.Value(updateID)},
			},
		}, updateID)
	}

	<-resetC
}

func TestDepthBuffer_ConcurrentRun(t *testing.T) {
	var snapshotFinalID int64 = 0
	buf := NewDepthBuffer("", func() (types.SliceOrderBook, int64, error) {
		snapshotFinalID += 30
		time.Sleep(10 * time.Millisecond)
		return types.SliceOrderBook{
			Bids: types.PriceVolumeSlice{
				{Price: 100, Volume: 1},
			},
			Asks: types.PriceVolumeSlice{
				{Price: 99, Volume: 1},
			},
		}, snapshotFinalID, nil
	})

	readyCnt := 0
	resetCnt := 0
	pushCnt := 0

	buf.OnPush(func(update Update) {
		pushCnt++
	})
	buf.OnReady(func(snapshot types.SliceOrderBook, updates []Update) {
		readyCnt++
		assert.Greater(t, len(updates), 0)
	})
	buf.OnReset(func() {
		resetCnt++
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	var updateID int64 = 10

	for {
		select {
		case <-ctx.Done():
			assert.Greater(t, readyCnt, 1)
			assert.Greater(t, resetCnt, 1)
			assert.Greater(t, pushCnt, 1)
			return

		case <-ticker.C:
			updateID++
			if updateID%100 == 0 {
				updateID++
			}

			buf.AddUpdate(types.SliceOrderBook{
				Bids: types.PriceVolumeSlice{
					{Price: 100, Volume: fixedpoint.Value(updateID)},
				},
				Asks: types.PriceVolumeSlice{
					{Price: 99, Volume: fixedpoint.Value(updateID)},
				},
			}, updateID)

		}
	}
}
