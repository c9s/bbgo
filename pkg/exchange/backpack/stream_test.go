package backpack

import (
	"bufio"
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/backpack/backpackapi"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

const publicStreamRecordFile = "backpackapi/testdata/backpack_ws_raw.jsonl"

const privateStreamRecordFile = "backpackapi/testdata/backpack_ws_private_raw.jsonl"

// newTestStream builds a stream that is not connected to anything.
func newTestStream(t *testing.T, publicOnly bool) *Stream {
	t.Helper()

	key, secret := newTestKeyPair()

	ex, err := New(key, secret)
	if err != nil {
		t.Fatalf("unable to create the test exchange: %v", err)
	}

	stream := NewStream(ex, ex.client)
	if publicOnly {
		stream.SetPublicOnly()
	}

	return stream
}

// replayStreamRecord pushes every frame of a recorded session through the stream's parser and
// dispatcher, which is the same path a live connection takes after Read.
func replayStreamRecord(t *testing.T, stream *Stream, recordFile string) int {
	t.Helper()

	file, err := os.Open(recordFile)
	if err != nil {
		t.Fatalf("unable to open the record file: %v", err)
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var dispatched int
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		event, err := backpackapi.ParseWebsocketMessage(line)
		if !assert.NoError(t, err) {
			continue
		}

		if event == nil {
			continue
		}

		stream.dispatchEvent(event)
		dispatched++
	}

	assert.NoError(t, scanner.Err())
	return dispatched
}

func TestStream_convertSubscription(t *testing.T) {
	clearSymbolMaps(t)
	defer clearSymbolMaps(t)

	stream := newTestStream(t, true)

	cases := []struct {
		name         string
		subscription types.Subscription
		expected     string
		expectError  bool
	}{
		{
			name:         "book",
			subscription: types.Subscription{Channel: types.BookChannel, Symbol: "SOLUSDC"},
			expected:     "depth.SOL_USDC",
		},
		{
			name:         "book ticker",
			subscription: types.Subscription{Channel: types.BookTickerChannel, Symbol: "SOLUSDC"},
			expected:     "bookTicker.SOL_USDC",
		},
		{
			name:         "market trade",
			subscription: types.Subscription{Channel: types.MarketTradeChannel, Symbol: "SOLUSDC"},
			expected:     "trade.SOL_USDC",
		},
		{
			name:         "ticker",
			subscription: types.Subscription{Channel: types.TickerChannel, Symbol: "SOLUSDC"},
			expected:     "ticker.SOL_USDC",
		},
		{
			name: "kline carries the interval in the stream name",
			subscription: types.Subscription{
				Channel: types.KLineChannel,
				Symbol:  "SOLUSDC",
				Options: types.SubscribeOptions{Interval: types.Interval1m},
			},
			expected: "kline.1m.SOL_USDC",
		},
		{
			name: "kline with an interval backpack spells differently",
			subscription: types.Subscription{
				Channel: types.KLineChannel,
				Symbol:  "SOLUSDC",
				Options: types.SubscribeOptions{Interval: types.Interval1mo},
			},
			expected: "kline.1month.SOL_USDC",
		},
		{
			name: "kline with an unsupported interval",
			subscription: types.Subscription{
				Channel: types.KLineChannel,
				Symbol:  "SOLUSDC",
				Options: types.SubscribeOptions{Interval: types.Interval2w},
			},
			expectError: true,
		},
		{
			name:         "unsupported channel",
			subscription: types.Subscription{Channel: types.AggTradeChannel, Symbol: "SOLUSDC"},
			expectError:  true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stream, err := stream.convertSubscription(c.subscription)
			if c.expectError {
				assert.Error(t, err)
				return
			}

			if assert.NoError(t, err) {
				assert.Equal(t, c.expected, stream)
			}
		})
	}
}

func TestStream_convertSubscriptionForFutures(t *testing.T) {
	clearSymbolMaps(t)
	defer clearSymbolMaps(t)

	stream := newTestStream(t, true)
	stream.exchange.UseFutures()

	name, err := stream.convertSubscription(types.Subscription{
		Channel: types.BookTickerChannel,
		Symbol:  "SOLUSDC",
	})

	if assert.NoError(t, err) {
		assert.Equal(t, "bookTicker.SOL_USDC_PERP", name)
	}
}

// TestStream_replayPublicRecord replays the recorded public session through the stream and
// checks that the frames come out as the expected global types.
//
// This is the offline integration test for the public streams: it needs no credentials and no
// network.
func TestStream_replayPublicRecord(t *testing.T) {
	clearSymbolMaps(t)
	defer clearSymbolMaps(t)

	stream := newTestStream(t, true)

	var (
		bookTickers  []types.BookTicker
		marketTrades []types.Trade
		kLines       []types.KLine
		closedKLines []types.KLine
	)

	stream.OnBookTickerUpdate(func(bookTicker types.BookTicker) {
		bookTickers = append(bookTickers, bookTicker)
	})

	stream.OnMarketTrade(func(trade types.Trade) {
		marketTrades = append(marketTrades, trade)
	})

	stream.OnKLine(func(kline types.KLine) {
		kLines = append(kLines, kline)
	})

	stream.OnKLineClosed(func(kline types.KLine) {
		closedKLines = append(closedKLines, kline)
	})

	dispatched := replayStreamRecord(t, stream, publicStreamRecordFile)
	assert.NotZero(t, dispatched)
	t.Logf("dispatched %d events", dispatched)

	t.Run("book tickers", func(t *testing.T) {
		if !assert.NotEmpty(t, bookTickers) {
			return
		}

		for _, bookTicker := range bookTickers {
			// the local symbol must have been converted
			assert.Equal(t, "SOLUSDC", bookTicker.Symbol)
			assert.True(t, bookTicker.Buy.Sign() > 0)
			assert.True(t, bookTicker.Sell.Sign() > 0)
			assert.True(t, bookTicker.Buy.Compare(bookTicker.Sell) < 0)
		}

		t.Logf("%d book tickers, first: %+v", len(bookTickers), bookTickers[0])
	})

	t.Run("market trades", func(t *testing.T) {
		if !assert.NotEmpty(t, marketTrades) {
			return
		}

		for _, trade := range marketTrades {
			assert.Equal(t, "SOLUSDC", trade.Symbol)
			assert.Equal(t, types.ExchangeBackpack, trade.Exchange)
			assert.NotZero(t, trade.ID)
			assert.True(t, trade.Price.Sign() > 0)
			assert.True(t, trade.Quantity.Sign() > 0)
			// the payload carries no quote quantity, it has to be derived
			assert.Equal(t, trade.Price.Mul(trade.Quantity), trade.QuoteQuantity)
			// the taker side is the reverse of the maker side the payload reports
			assert.Equal(t, trade.IsMaker, trade.Side == types.SideTypeSell)
		}

		t.Logf("%d market trades, first: %+v", len(marketTrades), marketTrades[0])
	})

	t.Run("klines", func(t *testing.T) {
		all := append(append([]types.KLine{}, kLines...), closedKLines...)
		if !assert.NotEmpty(t, all) {
			return
		}

		for _, kline := range all {
			assert.Equal(t, "SOLUSDC", kline.Symbol)
			assert.Equal(t, types.ExchangeBackpack, kline.Exchange)
			// the interval is not in the payload, it comes from the stream name
			assert.Equal(t, types.Interval1m, kline.Interval)
			assert.False(t, kline.StartTime.Time().IsZero())
			// the end time follows the binance rule rather than the exclusive end the payload has
			assert.Equal(t,
				kline.StartTime.Time().Add(time.Minute-time.Millisecond),
				kline.EndTime.Time())
		}

		assert.NotEmpty(t, closedKLines, "the recording should contain at least one closed kline")
		t.Logf("%d open and %d closed klines", len(kLines), len(closedKLines))
	})
}

// TestStream_replayPrivateRecord replays the recorded private session, which covers the order
// life cycle of a post-only order that was placed and cancelled while recording.
func TestStream_replayPrivateRecord(t *testing.T) {
	clearSymbolMaps(t)
	defer clearSymbolMaps(t)

	stream := newTestStream(t, false)

	var (
		orderUpdates   []types.Order
		balanceUpdates []types.BalanceMap
		tradeUpdates   []types.Trade
	)

	stream.OnOrderUpdate(func(order types.Order) {
		orderUpdates = append(orderUpdates, order)
	})

	stream.OnBalanceUpdate(func(balances types.BalanceMap) {
		balanceUpdates = append(balanceUpdates, balances)
	})

	stream.OnTradeUpdate(func(trade types.Trade) {
		tradeUpdates = append(tradeUpdates, trade)
	})

	dispatched := replayStreamRecord(t, stream, privateStreamRecordFile)
	assert.NotZero(t, dispatched)

	t.Run("order updates", func(t *testing.T) {
		if !assert.NotEmpty(t, orderUpdates) {
			return
		}

		for _, order := range orderUpdates {
			assert.Equal(t, "USDTUSDC", order.Symbol)
			assert.Equal(t, types.ExchangeBackpack, order.Exchange)
			assert.Equal(t, types.SideTypeSell, order.Side)
			// the order was submitted post-only, which maps to a limit maker order
			assert.Equal(t, types.OrderTypeLimitMaker, order.Type)
			assert.NotZero(t, order.OrderID)
			assert.NotEmpty(t, order.UUID)
			assert.True(t, order.Price.Sign() > 0)
		}

		// the recording covers an order that was accepted and then cancelled
		var statuses []types.OrderStatus
		for _, order := range orderUpdates {
			statuses = append(statuses, order.Status)
		}

		assert.Contains(t, statuses, types.OrderStatusNew)
		assert.Contains(t, statuses, types.OrderStatusCanceled)

		t.Logf("%d order updates, statuses: %v", len(orderUpdates), statuses)
	})

	t.Run("balance updates", func(t *testing.T) {
		if !assert.NotEmpty(t, balanceUpdates) {
			return
		}

		for _, balances := range balanceUpdates {
			for currency, balance := range balances {
				assert.NotEmpty(t, currency)
				assert.False(t, balance.Available.Sign() < 0)
				assert.False(t, balance.Locked.Sign() < 0)
			}
		}

		// placing the order locks the funds and cancelling it releases them again
		first := balanceUpdates[0]["USDT"]
		last := balanceUpdates[len(balanceUpdates)-1]["USDT"]
		assert.True(t, first.Locked.Sign() > 0, "the funds should be locked while the order rests")
		assert.True(t, last.Locked.IsZero(), "the funds should be released after the cancel")

		t.Logf("%d balance updates, first: %+v, last: %+v", len(balanceUpdates), first, last)
	})

	t.Run("no fills", func(t *testing.T) {
		// the probe order rests away from the market, so it must never fill
		assert.Empty(t, tradeUpdates)
	})
}

// TestStream_liveConnect connects to the real websocket and waits for market data.
//
// It needs no credentials, only network access, and is skipped unless TEST_BACKPACK_WS_LIVE is
// set so that CI stays offline.
func TestStream_liveConnect(t *testing.T) {
	if os.Getenv("TEST_BACKPACK_WS_LIVE") == "" {
		t.Skip("TEST_BACKPACK_WS_LIVE is not set, skipping the live websocket test")
	}

	clearSymbolMaps(t)
	defer clearSymbolMaps(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream := newTestStream(t, true)
	stream.Subscribe(types.BookTickerChannel, "SOLUSDC", types.SubscribeOptions{})
	stream.Subscribe(types.MarketTradeChannel, "SOLUSDC", types.SubscribeOptions{})
	stream.Subscribe(types.BookChannel, "SOLUSDC", types.SubscribeOptions{})

	bookTickerC := make(chan types.BookTicker, 1)
	stream.OnBookTickerUpdate(func(bookTicker types.BookTicker) {
		select {
		case bookTickerC <- bookTicker:
		default:
		}
	})

	snapshotC := make(chan types.SliceOrderBook, 1)
	stream.OnBookSnapshot(func(book types.SliceOrderBook) {
		select {
		case snapshotC <- book:
		default:
		}
	})

	if err := stream.Connect(ctx); err != nil {
		t.Fatalf("unable to connect: %v", err)
	}

	defer stream.Close()

	select {
	case bookTicker := <-bookTickerC:
		assert.Equal(t, "SOLUSDC", bookTicker.Symbol)
		assert.True(t, bookTicker.Buy.Compare(bookTicker.Sell) < 0)
		t.Logf("book ticker: %+v", bookTicker)

	case <-ctx.Done():
		t.Fatal("no book ticker received")
	}

	// the order book needs a REST snapshot before it can emit, so give it more room
	select {
	case book := <-snapshotC:
		assert.Equal(t, "SOLUSDC", book.Symbol)
		assert.NotEmpty(t, book.Bids)
		assert.NotEmpty(t, book.Asks)

		bestBid, ok := book.BestBid()
		assert.True(t, ok)

		bestAsk, ok := book.BestAsk()
		assert.True(t, ok)
		assert.True(t, bestBid.Price.Compare(bestAsk.Price) < 0)

		t.Logf("order book snapshot: %d bids, %d asks, best %s/%s",
			len(book.Bids), len(book.Asks), bestBid.Price, bestAsk.Price)

	case <-ctx.Done():
		t.Fatal("no order book snapshot received")
	}
}

// TestStream_livePrivateConnect connects the user data stream and verifies that the
// authentication is accepted.
func TestStream_livePrivateConnect(t *testing.T) {
	if os.Getenv("TEST_BACKPACK_WS_LIVE") == "" {
		t.Skip("TEST_BACKPACK_WS_LIVE is not set, skipping the live websocket test")
	}

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BACKPACK")
	if !ok {
		t.Skip("BACKPACK api key is not configured")
	}

	clearSymbolMaps(t)
	defer clearSymbolMaps(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ex, err := New(key, secret)
	if err != nil {
		t.Fatalf("unable to create the exchange: %v", err)
	}

	stream := NewStream(ex, ex.client)

	authC := make(chan struct{}, 1)
	stream.OnAuth(func() {
		select {
		case authC <- struct{}{}:
		default:
		}
	})

	balanceC := make(chan types.BalanceMap, 1)
	stream.OnBalanceSnapshot(func(balances types.BalanceMap) {
		select {
		case balanceC <- balances:
		default:
		}
	})

	// a rejected subscribe is reported as an error frame rather than a failed connect
	errorC := make(chan *backpackapi.WebsocketError, 1)
	stream.OnErrorEvent(func(e *backpackapi.WebsocketError) {
		select {
		case errorC <- e:
		default:
		}
	})

	if err := stream.Connect(ctx); err != nil {
		t.Fatalf("unable to connect: %v", err)
	}

	defer stream.Close()

	select {
	case <-authC:
		t.Log("authenticated")
	case <-ctx.Done():
		t.Fatal("no auth event received")
	}

	select {
	case balances := <-balanceC:
		assert.NotNil(t, balances)
		t.Logf("balance snapshot: %+v", balances)
	case <-ctx.Done():
		t.Fatal("no balance snapshot received")
	}

	// give the server a moment to reject the subscription if the signature was wrong
	select {
	case e := <-errorC:
		t.Fatalf("the server rejected the subscription: %v", e)
	case <-time.After(5 * time.Second):
	}
}
