package okex

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

func getTestClientOrSkip(t *testing.T) *Stream {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")
	if !ok {
		t.Skip("OKEX_* env vars are not configured")
		return nil
	}

	exchange := New(key, secret, passphrase)
	return NewStream(exchange.client, exchange)
}

func TestStream(t *testing.T) {
	t.Skip()
	s := getTestClientOrSkip(t)

	t.Run("account test", func(t *testing.T) {
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnBalanceUpdate(func(balances types.BalanceMap) {
			t.Log("got snapshot", balances)
		})
		s.OnBalanceSnapshot(func(balances types.BalanceMap) {
			t.Log("got snapshot", balances)
		})
		s.OnBookUpdate(func(book types.SliceOrderBook) {
			t.Log("got update", book)
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("book test", func(t *testing.T) {
		s.Subscribe(types.BookChannel, "BTCUSDT", types.SubscribeOptions{
			Depth: types.DepthLevel400,
		})
		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnBookSnapshot(func(book types.SliceOrderBook) {
			t.Log("got snapshot", book)
		})
		s.OnBookUpdate(func(book types.SliceOrderBook) {
			t.Log("got update", book)
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("book && kline test", func(t *testing.T) {
		s.Subscribe(types.BookChannel, "BTCUSDT", types.SubscribeOptions{
			Depth: types.DepthLevel400,
		})
		s.Subscribe(types.KLineChannel, "BTCUSDT", types.SubscribeOptions{
			Interval: types.Interval1m,
		})
		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnBookSnapshot(func(book types.SliceOrderBook) {
			t.Log("got snapshot", book)
		})
		s.OnBookUpdate(func(book types.SliceOrderBook) {
			t.Log("got update", book)
		})
		s.OnKLine(func(kline types.KLine) {
			t.Log("kline", kline)
		})
		s.OnKLineClosed(func(kline types.KLine) {
			t.Log("kline closed", kline)
		})

		c := make(chan struct{})
		<-c
	})

	t.Run("market trade test", func(t *testing.T) {
		s.Subscribe(types.MarketTradeChannel, "BTCUSDT", types.SubscribeOptions{})
		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnMarketTrade(func(trade types.Trade) {
			t.Log("got trade upgrade", trade)
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("kline test", func(t *testing.T) {
		s.Subscribe(types.KLineChannel, "LTC-USD-200327", types.SubscribeOptions{
			Interval: types.Interval1m,
		})
		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnKLine(func(kline types.KLine) {
			t.Log("got update", kline)
		})
		s.OnKLineClosed(func(kline types.KLine) {
			t.Log("got closed", kline)
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("Subscribe/Unsubscribe test", func(t *testing.T) {
		s.Subscribe(types.BookChannel, "BTCUSDT", types.SubscribeOptions{
			Depth: types.DepthLevel400,
		})
		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnBookSnapshot(func(book types.SliceOrderBook) {
			t.Log("got snapshot", book)
		})
		s.OnBookUpdate(func(book types.SliceOrderBook) {
			t.Log("got update", book)
		})

		<-time.After(5 * time.Second)

		s.Unsubscribe()
		c := make(chan struct{})
		<-c
	})

	t.Run("Resubscribe test", func(t *testing.T) {
		s.Subscribe(types.BookChannel, "BTCUSDT", types.SubscribeOptions{
			Depth: types.DepthLevel400,
		})
		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnBookSnapshot(func(book types.SliceOrderBook) {
			t.Log("got snapshot", book)
		})
		s.OnBookUpdate(func(book types.SliceOrderBook) {
			t.Log("got update", book)
		})

		<-time.After(5 * time.Second)

		s.Resubscribe(func(old []types.Subscription) (new []types.Subscription, err error) {
			return old, nil
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("order trade test", func(t *testing.T) {
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnOrderUpdate(func(order types.Order) {
			t.Log("order update", order)
		})
		s.OnTradeUpdate(func(trade types.Trade) {
			t.Log("trade update", trade)
		})
		c := make(chan struct{})
		<-c
	})
}

func TestUseFutures(t *testing.T) {
	s := getTestClientOrSkip(t)
	s.UseFutures()

	err := s.Connect(context.Background())
	assert.NoError(t, err)
	assert.True(t, s.isFutures)
}

func TestSubscribePrivateChannels(t *testing.T) {
	s := getTestClientOrSkip(t)

	// Test normal case
	t.Run("normal case", func(t *testing.T) {
		// Create a channel to track if the next function is called
		nextCalled := make(chan struct{})

		// Get the function returned by subscribePrivateChannels
		subscribeFunc := s.subscribePrivateChannels(func() {
			close(nextCalled)
		})

		// Connect to the exchange
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		// Call the returned function
		subscribeFunc()

		// Wait for the next function to be called or timeout
		select {
		case <-nextCalled:
			// next function was called, test passed
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for next function to be called")
		}
	})

	// Test futures mode
	t.Run("futures mode", func(t *testing.T) {
		s := getTestClientOrSkip(t)
		s.UseFutures()

		// Create a channel to track if the next function is called
		nextCalled := make(chan struct{})

		// Get the function returned by subscribePrivateChannels
		subscribeFunc := s.subscribePrivateChannels(func() {
			close(nextCalled)
		})

		// Connect to the exchange
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		// Call the returned function
		subscribeFunc()

		// Wait for the next function to be called or timeout
		select {
		case <-nextCalled:
			// next function was called, test passed
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for next function to be called")
		}

		// Verify that the isFutures flag is correctly set
		assert.True(t, s.isFutures)
	})
}
