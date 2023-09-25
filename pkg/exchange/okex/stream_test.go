package okex

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

func getTestClientOrSkip(t *testing.T) *Stream {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")
	if !ok {
		t.Skip("Please configure all credentials about OKEX")
		return nil
	}

	return NewStream(key, secret, passphrase)
}

func TestStream(t *testing.T) {
	// t.Skip()
	s := getTestClientOrSkip(t)

	t.Run("Auth test", func(t *testing.T) {
		s.Connect(context.Background())
		c := make(chan struct{})
		<-c
	})

	t.Run("account test", func(t *testing.T) {
		s.OnAuth(func() {
			fmt.Println("authenticated")
		})
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnAccountEvent(func(account okexapi.Account) {
			fmt.Println("account detail", account)
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("book test", func(t *testing.T) {
		s.Subscribe(types.BookChannel, "BTCUSDT", types.SubscribeOptions{})
		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnBookSnapshot(func(book types.SliceOrderBook) {
			fmt.Println()
			fmt.Printf("book detail: %+v", book)
		})
		s.OnBookUpdate(func(book types.SliceOrderBook) {
			fmt.Println()
			fmt.Printf("book update detail: %+v", book)
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("book ticker test", func(t *testing.T) {
		s.Subscribe(types.BookTickerChannel, "BTCUSDT", types.SubscribeOptions{})
		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnBookTickerUpdate(func(bookTicker types.BookTicker) {
			fmt.Println()
			fmt.Printf("bookticker detail: %+v", bookTicker)
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("kline test", func(t *testing.T) {
		s.Subscribe(types.KLineChannel, "LTCUSDT", types.SubscribeOptions{
			Interval: types.Interval30m,
			Depth:    "",
			Speed:    "",
		})
		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnKLine(func(kline types.KLine) {
			fmt.Println()
			fmt.Printf("kline (candle) detail: %+v", kline)
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("order test", func(t *testing.T) {
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnOrderUpdate(func(order types.Order) {
			fmt.Println()
			fmt.Printf("order update detail: %+v", order)
		})
		c := make(chan struct{})
		<-c
	})

}
