package component

import (
	"context"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

func Test_StreamBookHealthCheck(t *testing.T) {
	t.Run("reconnects when order book is stale", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		book := types.NewStreamBook("BTCUSDT", types.ExchangeMax)

		mockStream := mocks.NewMockStream(mockCtrl)
		reconnectCalled := make(chan struct{}, 1)
		mockStream.EXPECT().Reconnect().Do(func() {
			select {
			case reconnectCalled <- struct{}{}:
			default:
			}
		}).AnyTimes()
		book.Stream = mockStream

		// Load data with an old timestamp to simulate stale data
		staleTime := time.Now().Add(-100 * time.Millisecond)
		book.Load(types.SliceOrderBook{
			Symbol: "BTCUSDT",
			Time:   staleTime,
			Bids: types.PriceVolumeSlice{
				{Price: fixedpoint.NewFromFloat(100.0), Volume: fixedpoint.NewFromFloat(1.0)},
			},
			Asks: types.PriceVolumeSlice{
				{Price: fixedpoint.NewFromFloat(101.0), Volume: fixedpoint.NewFromFloat(1.0)},
			},
		})

		// Set up health check with short durations for testing
		checkDuration := 10 * time.Millisecond
		reconnectThreshold := 50 * time.Millisecond

		book.Use(StreamBookHealthCheck(ctx, "", checkDuration, reconnectThreshold))

		// Wait for health check to trigger reconnect
		select {
		case <-reconnectCalled:
			// Success - reconnect was called
		case <-time.After(500 * time.Millisecond):
			t.Fatal("expected Reconnect to be called but it wasn't")
		}
	})

	t.Run("does not reconnect when order book is fresh", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		book := types.NewStreamBook("BTCUSDT", types.ExchangeMax)

		mockStream := mocks.NewMockStream(mockCtrl)
		reconnectCalled := make(chan struct{}, 1)
		mockStream.EXPECT().Reconnect().Do(func() {
			select {
			case reconnectCalled <- struct{}{}:
			default:
			}
		}).Times(0)
		book.Stream = mockStream

		checkDuration := 10 * time.Millisecond
		reconnectThreshold := 200 * time.Millisecond

		book.Use(StreamBookHealthCheck(ctx, "", checkDuration, reconnectThreshold))

		// Keep updating the book to keep it fresh
		go func() {
			ticker := time.NewTicker(20 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					book.Load(types.SliceOrderBook{
						Symbol: "BTCUSDT",
						Time:   time.Now(),
						Bids: types.PriceVolumeSlice{
							{Price: fixedpoint.NewFromFloat(100.0), Volume: fixedpoint.NewFromFloat(1.0)},
						},
						Asks: types.PriceVolumeSlice{
							{Price: fixedpoint.NewFromFloat(101.0), Volume: fixedpoint.NewFromFloat(1.0)},
						},
					})
				}
			}
		}()

		// Wait enough time for multiple health checks
		select {
		case <-reconnectCalled:
			t.Fatal("Reconnect should not have been called for fresh order book")
		case <-time.After(100 * time.Millisecond):
			// Success - reconnect was not called
		}
	})

	t.Run("does not reconnect when stream is nil", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		book := types.NewStreamBook("BTCUSDT", types.ExchangeMax)
		// Do not set book.Stream

		// Load stale data
		staleTime := time.Now().Add(-100 * time.Millisecond)
		book.Load(types.SliceOrderBook{
			Symbol: "BTCUSDT",
			Time:   staleTime,
			Bids: types.PriceVolumeSlice{
				{Price: fixedpoint.NewFromFloat(100.0), Volume: fixedpoint.NewFromFloat(1.0)},
			},
			Asks: types.PriceVolumeSlice{
				{Price: fixedpoint.NewFromFloat(101.0), Volume: fixedpoint.NewFromFloat(1.0)},
			},
		})

		checkDuration := 10 * time.Millisecond
		reconnectThreshold := 50 * time.Millisecond

		// This should not panic even though stream is nil
		book.Use(StreamBookHealthCheck(ctx, "", checkDuration, reconnectThreshold))

		// Wait for health check to run (it should not panic)
		time.Sleep(100 * time.Millisecond)
	})
}
