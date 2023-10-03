package okex

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

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
	e := New(key, secret, passphrase)

	return NewStream(e.client)
}

func TestStream(t *testing.T) {

	s := getTestClientOrSkip(t)

	ctx, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()

	deadline := time.Now().Add(5 * time.Second)

	t.Run("Auth test", func(t *testing.T) {
		s.Connect(ctx)
		assert.False(t, s.PublicOnly)
	})

	t.Run("account test", func(t *testing.T) {
		s.OnAuth(func() {
			fmt.Println("authenticated")
		})
		err := s.Connect(ctx)
		assert.NoError(t, err)
		var accountVerify okexapi.Account

		s.OnAccountEvent(func(account okexapi.Account) {
			accountVerify = account
		})

		for {
			if len(accountVerify.Details) != 0 || time.Now().After(deadline) {
				assert.NotEmpty(t, accountVerify.Details)
				break
			}
		}
	})

	t.Run("book test", func(t *testing.T) {
		s.Subscribe(types.BookChannel, "BTCUSDT", types.SubscribeOptions{
			Depth: types.DepthLevel400,
		})
		s.SetPublicOnly()
		err := s.Connect(ctx)
		assert.NoError(t, err)
		var sliceOrderBook types.SliceOrderBook
		var updateSliceOrderBook types.SliceOrderBook

		s.OnBookSnapshot(func(book types.SliceOrderBook) {
			sliceOrderBook = book
		})
		s.OnBookUpdate(func(book types.SliceOrderBook) {
			updateSliceOrderBook = book
		})
		for {
			if len(sliceOrderBook.Asks) != 0 || time.Now().After(deadline) {
				assert.NotEmpty(t, sliceOrderBook.Asks)
				assert.NotEmpty(t, sliceOrderBook.Bids)
				break
			}
		}
		for {
			if len(updateSliceOrderBook.Asks) != 0 || time.Now().After(deadline) {
				assert.NotEmpty(t, updateSliceOrderBook.Asks)
				assert.NotEmpty(t, updateSliceOrderBook.Bids)
				break
			}
		}

	})

	t.Run("book ticker test", func(t *testing.T) {
		s.Subscribe(types.BookTickerChannel, "BTCUSDT", types.SubscribeOptions{})
		s.SetPublicOnly()
		err := s.Connect(ctx)
		assert.NoError(t, err)
		var bookticker types.BookTicker

		s.OnBookTickerUpdate(func(bookTicker types.BookTicker) {
			bookticker = bookTicker
		})
		for {
			if bookticker.Symbol != "" || time.Now().After(deadline) {
				assert.True(t, bookticker.Symbol != "")
				assert.NotEmpty(t, bookticker.Buy)
				break
			}
		}

	})

	t.Run("kline test", func(t *testing.T) {
		s.Subscribe(types.KLineChannel, "LTCUSDT", types.SubscribeOptions{
			Interval: types.Interval30m,
			Depth:    "",
			Speed:    "",
		})
		s.SetPublicOnly()
		err := s.Connect(ctx)
		assert.Error(t, err)
		fmt.Printf("get error message: %s", err)
	})

	// Only order Updated would trigger order channel
	t.Run("order test", func(t *testing.T) {
		err := s.Connect(ctx)
		assert.NoError(t, err)
		var o types.Order

		s.OnOrderUpdate(func(order types.Order) {
			o = order
		})

		for {
			if o.Exchange != "" || time.Now().After(deadline) {
				assert.True(t, o.Exchange == "")
				assert.True(t, o.OrderID == 0)
				break
			}
		}

	})

}
