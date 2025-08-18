//go:build !race
// +build !race

package depth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var itov = fixedpoint.NewFromInt

func TestDepthBuffer_ReadyState(t *testing.T) {
	buf := NewBuffer(func() (book types.SliceOrderBook, finalID int64, err error) {
		return types.SliceOrderBook{
			Bids: types.PriceVolumeSlice{
				{Price: itov(100), Volume: itov(1)},
			},
			Asks: types.PriceVolumeSlice{
				{Price: itov(99), Volume: itov(1)},
			},
		}, 33, nil
	}, time.Millisecond*5)

	readyC := make(chan struct{})
	buf.OnReady(func(snapshot types.SliceOrderBook, updates []Update) {
		assert.Greater(t, len(updates), 33)
		close(readyC)
	})

	var updateID int64 = 1
	for ; updateID < 100; updateID++ {
		err := buf.AddUpdate(
			types.SliceOrderBook{
				Bids: types.PriceVolumeSlice{
					{Price: itov(100), Volume: itov(updateID)},
				},
				Asks: types.PriceVolumeSlice{
					{Price: itov(99), Volume: itov(updateID)},
				},
			}, updateID)

		assert.NoError(t, err)
	}

	select {
	case <-readyC:
	case <-time.After(time.Minute):
		t.Fail()
	}
}

func TestDepthBuffer_CorruptedUpdateAtTheBeginning(t *testing.T) {
	// snapshot starts from 30,
	// the first ready event should have a snapshot(30) and updates (31~50)
	var snapshotFinalID int64 = 0
	buf := NewBuffer(func() (types.SliceOrderBook, int64, error) {
		snapshotFinalID += 30
		return types.SliceOrderBook{
			Bids: types.PriceVolumeSlice{
				{Price: itov(100), Volume: itov(1)},
			},
			Asks: types.PriceVolumeSlice{
				{Price: itov(99), Volume: itov(1)},
			},
		}, snapshotFinalID, nil
	}, time.Millisecond)

	resetC := make(chan struct{}, 2)

	buf.OnReset(func() {
		t.Logf("reset triggered")
		resetC <- struct{}{}
	})

	var updateID int64 = 10
	for ; updateID < 100; updateID++ {
		time.Sleep(time.Millisecond)

		// send corrupt update when updateID = 50
		if updateID == 50 {
			updateID += 5
		}

		err := buf.AddUpdate(types.SliceOrderBook{
			Bids: types.PriceVolumeSlice{
				{Price: itov(100), Volume: itov(updateID)},
			},
			Asks: types.PriceVolumeSlice{
				{Price: itov(99), Volume: itov(updateID)},
			},
		}, updateID)

		if err != nil {
			t.Log("emit reset")
			buf.Reset()
		}
	}

	select {
	case <-resetC:
	case <-time.After(10 * time.Second):
		t.Fail()
	}
}

func TestDepthBuffer_ConcurrentRun(t *testing.T) {
	var snapshotFinalID int64 = 0
	buf := NewBuffer(func() (types.SliceOrderBook, int64, error) {
		snapshotFinalID += 30
		time.Sleep(10 * time.Millisecond)
		return types.SliceOrderBook{
			Bids: types.PriceVolumeSlice{
				{Price: itov(100), Volume: itov(1)},
			},
			Asks: types.PriceVolumeSlice{
				{Price: itov(99), Volume: itov(1)},
			},
		}, snapshotFinalID, nil
	}, time.Millisecond*5)

	readyCnt := 0
	resetCnt := 0
	pushCnt := 0

	buf.OnPush(func(update Update) {
		pushCnt++
	})
	buf.OnReady(func(snapshot types.SliceOrderBook, updates []Update) {
		readyCnt++
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
					{Price: itov(100), Volume: itov(updateID)},
				},
				Asks: types.PriceVolumeSlice{
					{Price: itov(99), Volume: itov(updateID)},
				},
			}, updateID)

		}
	}
}

func TestDepthBuffer_FuturesReadyState(t *testing.T) {
	buf := NewBuffer(func() (book types.SliceOrderBook, finalID int64, err error) {
		return types.SliceOrderBook{
			Bids: types.PriceVolumeSlice{
				{Price: itov(100), Volume: itov(1)},
			},
			Asks: types.PriceVolumeSlice{
				{Price: itov(99), Volume: itov(1)},
			},
		}, 33, nil
	}, time.Millisecond*5)

	buf.CheckPreviousID()

	readyC := make(chan struct{})
	buf.OnReady(func(snapshot types.SliceOrderBook, updates []Update) {
		assert.Greater(t, len(updates), 33)
		close(readyC)
	})

	var updateID int64 = 1
	for ; updateID < 100; updateID++ {
		err := buf.AddUpdate(
			types.SliceOrderBook{
				Bids: types.PriceVolumeSlice{
					{Price: itov(100), Volume: itov(updateID)},
				},
				Asks: types.PriceVolumeSlice{
					{Price: itov(99), Volume: itov(updateID)},
				},
			}, updateID, updateID, updateID-1)

		assert.NoError(t, err)
	}

	select {
	case <-readyC:
	case <-time.After(time.Minute):
		t.Fail()
	}
}

func TestDepthBuffer_FuturesCorruptedUpdateAtTheBeginning(t *testing.T) {
	// snapshot starts from 30,
	// the first ready event should have a snapshot(30) and updates (31~50)
	var snapshotFinalID int64 = 0
	buf := NewBuffer(func() (types.SliceOrderBook, int64, error) {
		snapshotFinalID += 30
		return types.SliceOrderBook{
			Bids: types.PriceVolumeSlice{
				{Price: itov(100), Volume: itov(1)},
			},
			Asks: types.PriceVolumeSlice{
				{Price: itov(99), Volume: itov(1)},
			},
		}, snapshotFinalID, nil
	}, time.Millisecond)

	buf.CheckPreviousID()

	resetC := make(chan struct{}, 2)

	buf.OnReset(func() {
		t.Logf("reset triggered")
		resetC <- struct{}{}
	})

	var updateID int64 = 10
	for ; updateID < 100; updateID++ {
		time.Sleep(time.Millisecond)

		// send corrupt update when updateID = 50
		previousUpdateId := updateID - 1
		if updateID == 50 {
			previousUpdateId = updateID + 1
		}

		err := buf.AddUpdate(types.SliceOrderBook{
			Bids: types.PriceVolumeSlice{
				{Price: itov(100), Volume: itov(updateID)},
			},
			Asks: types.PriceVolumeSlice{
				{Price: itov(99), Volume: itov(updateID)},
			},
		}, updateID, updateID, previousUpdateId)

		if err != nil {
			t.Log("emit reset")
			buf.Reset()
		}
	}

	select {
	case <-resetC:
	case <-time.After(10 * time.Second):
		t.Fail()
	}
}
