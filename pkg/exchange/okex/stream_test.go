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
