package hyperliquid

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/exchange/hyperliquid/hyperapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

// Unit tests for WebSocket message parsing

func TestParseWebSocketEvent_SubscriptionResponse(t *testing.T) {
	t.Parallel()
	msg := `{"channel":"subscriptionResponse","data":{"method":"subscribe","subscription":{"type":"l2Book","coin":"BTC"}}}`

	event, err := parseWebSocketEvent([]byte(msg))
	require.NoError(t, err)
	require.NotNil(t, event)

	subResp, ok := event.(*SubscriptionResponseEvent)
	assert.True(t, ok)
	assert.NotNil(t, subResp.Data)
}

func TestParseWebSocketEvent_L2Book(t *testing.T) {
	t.Parallel()
	msg := `{
		"channel":"l2Book",
		"data":{
			"coin":"BTC",
			"time":1234567890,
			"levels":[
				[{"px":"50000.0","sz":"1.5","n":2}],
				[{"px":"50100.0","sz":"2.0","n":3}]
			]
		}
	}`

	event, err := parseWebSocketEvent([]byte(msg))
	require.NoError(t, err)
	require.NotNil(t, event)

	bookEvent, ok := event.(*WsBookEvent)
	assert.True(t, ok)
	assert.Equal(t, "BTC", bookEvent.Book.Coin)
	assert.Equal(t, int64(1234567890), bookEvent.Book.Time.Time().Unix())
	assert.Len(t, bookEvent.Book.Levels[0], 1) // bids
	assert.Len(t, bookEvent.Book.Levels[1], 1) // asks
}

func TestParseWebSocketEvent_Trades(t *testing.T) {
	t.Parallel()
	msg := `{
		"channel":"trades",
		"data":[
			{
				"coin":"BTC",
				"side":"B",
				"px":"50000.0",
				"sz":"0.5",
				"hash":"abc123",
				"time":1234567890,
				"tid":12345,
				"users":["user1","user2"]
			}
		]
	}`

	event, err := parseWebSocketEvent([]byte(msg))
	require.NoError(t, err)
	require.NotNil(t, event)

	tradesEvent, ok := event.(*WsTradesEvent)
	assert.True(t, ok)
	assert.Len(t, tradesEvent.Trades, 1)
	assert.Equal(t, "BTC", tradesEvent.Trades[0].Coin)
	assert.Equal(t, "B", tradesEvent.Trades[0].Side)
	assert.Equal(t, fixedpoint.NewFromFloat(50000.0), tradesEvent.Trades[0].Px)
}

func TestParseWebSocketEvent_Candle(t *testing.T) {
	t.Parallel()
	msg := `{
		"channel":"candle",
		"data":{
			"t":1234567890000,
			"T":1234567949999,
			"s":"BTC",
			"i":"1m",
			"o":50000.0,
			"c":50100.0,
			"h":50200.0,
			"l":49900.0,
			"v":100.5,
			"n":50
		}
	}`

	event, err := parseWebSocketEvent([]byte(msg))
	require.NoError(t, err)
	require.NotNil(t, event)

	candleEvent, ok := event.(*WsCandleEvent)
	assert.True(t, ok)
	assert.Equal(t, "BTC", candleEvent.Candle.Symbol)
	assert.Equal(t, "1m", candleEvent.Candle.Interval)
	assert.Equal(t, fixedpoint.NewFromFloat(50000.0), candleEvent.Candle.O)
	assert.Equal(t, fixedpoint.NewFromFloat(50100.0), candleEvent.Candle.C)
}

func TestParseWebSocketEvent_UserFills(t *testing.T) {
	t.Parallel()
	isSnapshot := false
	msg := `{
		"channel":"userFills",
		"data":{
			"user":"0x1234567890abcdef",
			"fills":[
				{
					"coin":"BTC",
					"px":"50000.0",
					"sz":"0.5",
					"side":"B",
					"time":1234567890,
					"startPosition":"0.0",
					"dir":"Open Long",
					"closedPnl":"0.0",
					"hash":"abc123",
					"oid":12345,
					"crossed":true,
					"fee":"5.0",
					"tid":67890,
					"feeToken":"USDC"
				}
			]
		},
		"isSnapshot":false
	}`

	event, err := parseWebSocketEvent([]byte(msg))
	require.NoError(t, err)
	require.NotNil(t, event)

	fillsEvent, ok := event.(*WsUserFillsEvent)
	assert.True(t, ok)
	assert.Equal(t, &isSnapshot, fillsEvent.UserFills.IsSnapshot)
	assert.Len(t, fillsEvent.UserFills.Fills, 1)
	assert.Equal(t, "BTC", fillsEvent.UserFills.Fills[0].Coin)
	assert.Equal(t, int64(12345), fillsEvent.UserFills.Fills[0].Oid)
}

func TestParseWebSocketEvent_OrderUpdate(t *testing.T) {
	t.Parallel()
	msg := `{
		"channel":"orderUpdates",
		"data":{
			"order":{
				"coin":"BTC",
				"side":"B",
				"limitPx":"50000.0",
				"sz":"0.5",
				"oid":12345,
				"timestamp":1234567890,
				"origSz":"1.0",
				"cloid":"client123"
			},
			"status":"open",
			"statusTimestamp":1234567890
		}
	}`

	event, err := parseWebSocketEvent([]byte(msg))
	require.NoError(t, err)
	require.NotNil(t, event)

	orderEvent, ok := event.(*WsOrderUpdateEvent)
	assert.True(t, ok)
	assert.Equal(t, "BTC", orderEvent.OrderUpdate.Order.Coin)
	assert.Equal(t, "open", orderEvent.OrderUpdate.Status)
	assert.Equal(t, int64(12345), orderEvent.OrderUpdate.Order.Oid)
}

func TestParseWebSocketEvent_ClearinghouseState(t *testing.T) {
	t.Parallel()
	msg := `{
		"channel":"clearinghouseState",
		"data":{
			"assetPositions":[
				{
					"type":"oneWay",
					"position":{
						"coin":"BTC",
						"szi":"1.5",
						"entryPx":"50000.0",
						"positionValue":"75000.0",
						"leverage":{"type":"cross","value":10.0},
						"liquidationPx":45000.0,
						"marginUsed":"7500.0",
						"unrealizedPnl":"1000.0"
					}
				}
			],
			"marginSummary":{
				"accountValue":100000.0,
				"totalNtlPos":75000.0,
				"totalRawUsd":100000.0,
				"totalMarginUsed":7500.0
			},
			"withdrawable":92500.0
		}
	}`

	event, err := parseWebSocketEvent([]byte(msg))
	require.NoError(t, err)
	require.NotNil(t, event)

	stateEvent, ok := event.(*WsClearinghouseStateEvent)
	assert.True(t, ok)
	assert.Len(t, stateEvent.State.AssetPositions, 1)
	assert.Equal(t, "BTC", stateEvent.State.AssetPositions[0].Position.Coin)
	assert.Equal(t, fixedpoint.NewFromFloat(100000.0), stateEvent.State.MarginSummary.AccountValue)
}

func TestParseWebSocketEvent_UnknownChannel(t *testing.T) {
	t.Parallel()
	msg := `{"channel":"unknownChannel","data":{}}`

	event, err := parseWebSocketEvent([]byte(msg))
	require.NoError(t, err)
	assert.Nil(t, event) // Unknown channels return nil
}

// Unit tests for conversion functions

func TestWsBookToSliceOrderBook(t *testing.T) {
	t.Parallel()

	book := WsBook{
		Coin: "BTC",
		Time: types.NewMillisecondTimestampFromInt(1234567890),
		Levels: [2][]WsLevel{
			{ // bids
				{Px: fixedpoint.NewFromFloat(50000.0), Sz: fixedpoint.NewFromFloat(1.5), N: 2},
				{Px: fixedpoint.NewFromFloat(49900.0), Sz: fixedpoint.NewFromFloat(2.0), N: 3},
			},
			{ // asks
				{Px: fixedpoint.NewFromFloat(50100.0), Sz: fixedpoint.NewFromFloat(1.0), N: 1},
				{Px: fixedpoint.NewFromFloat(50200.0), Sz: fixedpoint.NewFromFloat(0.5), N: 1},
			},
		},
	}

	result := wsBookToSliceOrderBook(book)
	assert.Equal(t, "BTCUSDC", result.Symbol)
	assert.Len(t, result.Bids, 2)
	assert.Len(t, result.Asks, 2)
	assert.Equal(t, fixedpoint.NewFromFloat(50000.0), result.Bids[0].Price)
	assert.Equal(t, fixedpoint.NewFromFloat(50100.0), result.Asks[0].Price)
}

func TestWsTradeToTrade(t *testing.T) {
	t.Parallel()

	wsTrade := WsTrade{
		Coin: "BTC",
		Side: "B",
		Px:   fixedpoint.NewFromFloat(50000.0),
		Sz:   fixedpoint.NewFromFloat(0.5),
		Hash: "abc123",
		Time: types.NewMillisecondTimestampFromInt(1234567890),
		Tid:  12345,
	}

	result := wsTradeToTrade(wsTrade, true)
	assert.Equal(t, "BTCUSDC", result.Symbol)
	assert.Equal(t, types.SideTypeBuy, result.Side)
	assert.Equal(t, fixedpoint.NewFromFloat(50000.0), result.Price)
	assert.Equal(t, fixedpoint.NewFromFloat(0.5), result.Quantity)
	assert.True(t, result.IsFutures)
}

func TestWsCandleToKLine(t *testing.T) {
	t.Parallel()

	// Test with a closed candle (past time)
	pastTime := time.Now().Add(-2 * time.Minute).UnixMilli()
	candle := WsCandle{
		OpenTime:  types.NewMillisecondTimestampFromInt(pastTime),
		CloseTime: types.NewMillisecondTimestampFromInt(pastTime + 60000), // 1 minute later
		Symbol:    "BTC",
		Interval:  "1m",
		O:         fixedpoint.NewFromFloat(50000.0),
		C:         fixedpoint.NewFromFloat(50100.0),
		H:         fixedpoint.NewFromFloat(50200.0),
		L:         fixedpoint.NewFromFloat(49900.0),
		V:         fixedpoint.NewFromFloat(100.5),
		N:         50,
	}

	result := wsCandleToKLine(candle)
	assert.Equal(t, "BTCUSDC", result.Symbol)
	assert.Equal(t, types.Interval1m, result.Interval)
	assert.Equal(t, fixedpoint.NewFromFloat(50000.0), result.Open)
	assert.Equal(t, fixedpoint.NewFromFloat(50100.0), result.Close)
}

func TestWsFillToTrade(t *testing.T) {
	t.Parallel()

	fill := WsFill{
		Coin:          "BTC",
		Px:            fixedpoint.NewFromFloat(50000.0),
		Sz:            fixedpoint.NewFromFloat(0.5),
		Side:          "B",
		Time:          types.NewMillisecondTimestampFromInt(1234567890),
		StartPosition: "0.0",
		Dir:           "Open Long",
		ClosedPnl:     "0.0",
		Hash:          "abc123",
		Oid:           12345,
		Crossed:       true,
		Fee:           "5.0",
		Tid:           67890,
		FeeToken:      "USDC",
	}

	result := wsFillToTrade(fill, true)
	assert.Equal(t, "BTCUSDC", result.Symbol)
	assert.Equal(t, types.SideTypeBuy, result.Side)
	assert.Equal(t, uint64(12345), result.OrderID)
	assert.Equal(t, fixedpoint.NewFromFloat(5.0), result.Fee)
	assert.Equal(t, "USDC", result.FeeCurrency)
	assert.False(t, result.IsMaker) // crossed = true means taker
	assert.True(t, result.IsFutures)
}

func TestWsOrderUpdateToOrder(t *testing.T) {
	t.Parallel()

	orderUpdate := WsOrderUpdate{
		Order: WsBasicOrder{
			Coin:      "BTC",
			Side:      "B",
			LimitPx:   fixedpoint.NewFromFloat(50000.0),
			Sz:        fixedpoint.NewFromFloat(0.5), // remaining
			Oid:       12345,
			Timestamp: types.NewMillisecondTimestampFromInt(1234567890),
			OrigSz:    fixedpoint.NewFromFloat(1.0), // original
			Cloid:     "client123",
		},
		Status:          "open",
		StatusTimestamp: types.NewMillisecondTimestampFromInt(1234567890),
	}

	result := wsOrderUpdateToOrder(orderUpdate, true)
	assert.Equal(t, "BTCUSDC", result.Symbol)
	assert.Equal(t, types.SideTypeBuy, result.Side)
	assert.Equal(t, uint64(12345), result.OrderID)
	assert.Equal(t, types.OrderStatusNew, result.Status)
	// ExecutedQuantity should be origSz - sz = 1.0 - 0.5 = 0.5
	assert.Equal(t, fixedpoint.NewFromFloat(0.5), result.ExecutedQuantity)
	assert.True(t, result.IsFutures)
}

func TestWsClearinghouseStateToFuturesPositions(t *testing.T) {
	t.Parallel()

	liquidationPx := fixedpoint.NewFromFloat(45000.0)
	state := WsClearinghouseState{
		AssetPositions: []WsAssetPosition{
			{
				Type: "oneWay",
				Position: WsPosition{
					Coin:          "BTC",
					Szi:           fixedpoint.NewFromFloat(1.5),
					EntryPx:       fixedpoint.NewFromFloat(50000.0),
					PositionValue: fixedpoint.NewFromFloat(75000.0),
					Leverage: struct {
						Type  string           `json:"type"`
						Value fixedpoint.Value `json:"value"`
					}{
						Type:  "cross",
						Value: fixedpoint.NewFromFloat(10.0),
					},
					LiquidationPx: &liquidationPx,
					MarginUsed:    fixedpoint.NewFromFloat(7500.0),
					UnrealizedPnl: fixedpoint.NewFromFloat(1000.0),
				},
			},
		},
		MarginSummary: WsMarginSummary{
			AccountValue:    fixedpoint.NewFromFloat(100000.0),
			TotalNtlPos:     fixedpoint.NewFromFloat(75000.0),
			TotalRawUsd:     fixedpoint.NewFromFloat(100000.0),
			TotalMarginUsed: fixedpoint.NewFromFloat(7500.0),
		},
		Withdrawable: 92500.0,
	}

	result := wsClearinghouseStateToFuturesPositions(state)
	assert.Len(t, result, 1)

	posKey := types.NewPositionKey("BTCUSDC", types.PositionLong)
	pos, exists := result[posKey]
	assert.True(t, exists)
	assert.Equal(t, "BTCUSDC", pos.Symbol)
	assert.Equal(t, types.PositionLong, pos.PositionSide)
	assert.Equal(t, fixedpoint.NewFromFloat(1.5), pos.Base)
	assert.False(t, pos.Isolated) // cross leverage
	assert.NotNil(t, pos.PositionRisk)
	assert.Equal(t, fixedpoint.NewFromFloat(10.0), pos.PositionRisk.Leverage)
	assert.Equal(t, fixedpoint.NewFromFloat(45000.0), pos.PositionRisk.LiquidationPrice)
}

func TestWsClearinghouseStateToFuturesPositions_ShortPosition(t *testing.T) {
	t.Parallel()

	state := WsClearinghouseState{
		AssetPositions: []WsAssetPosition{
			{
				Type: "oneWay",
				Position: WsPosition{
					Coin:          "ETH",
					Szi:           fixedpoint.NewFromFloat(-2.5), // negative = short
					EntryPx:       fixedpoint.NewFromFloat(3000.0),
					PositionValue: fixedpoint.NewFromFloat(7500.0),
					Leverage: struct {
						Type  string           `json:"type"`
						Value fixedpoint.Value `json:"value"`
					}{
						Type:  "isolated",
						Value: fixedpoint.NewFromFloat(5.0),
					},
					LiquidationPx: nil, // test nil liquidation price
					MarginUsed:    fixedpoint.NewFromFloat(1500.0),
					UnrealizedPnl: fixedpoint.NewFromFloat(-100.0),
				},
			},
		},
	}

	result := wsClearinghouseStateToFuturesPositions(state)
	assert.Len(t, result, 1)

	posKey := types.NewPositionKey("ETHUSDC", types.PositionShort)
	pos, exists := result[posKey]
	assert.True(t, exists)
	assert.Equal(t, types.PositionShort, pos.PositionSide)
	assert.Equal(t, fixedpoint.NewFromFloat(2.5), pos.Base) // absolute value
	assert.True(t, pos.Isolated)
	assert.Equal(t, fixedpoint.Zero, pos.PositionRisk.LiquidationPrice) // nil -> zero
}

func TestWsClearinghouseStateToFuturesPositions_ZeroPosition(t *testing.T) {
	t.Parallel()

	state := WsClearinghouseState{
		AssetPositions: []WsAssetPosition{
			{
				Type: "oneWay",
				Position: WsPosition{
					Coin:          "BTC",
					Szi:           fixedpoint.NewFromFloat(0.0), // zero position
					EntryPx:       fixedpoint.NewFromFloat(50000.0),
					PositionValue: fixedpoint.NewFromFloat(0.0),
					Leverage: struct {
						Type  string           `json:"type"`
						Value fixedpoint.Value `json:"value"`
					}{
						Type:  "cross",
						Value: fixedpoint.NewFromFloat(10.0),
					},
					MarginUsed:    fixedpoint.NewFromFloat(0.0),
					UnrealizedPnl: fixedpoint.NewFromFloat(0.0),
				},
			},
		},
	}

	result := wsClearinghouseStateToFuturesPositions(state)
	assert.Len(t, result, 0) // Zero positions should be skipped
}

// Integration tests (require real credentials)

func getTestClientOrSkip(t *testing.T) *Stream {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

	secret, account, _, ok := testutil.IntegrationTestWithPrivateKeyConfigured(t, "HYPERLIQUID")
	if !ok {
		t.Skip("HYPERLIQUID_* env vars are not configured")
		return nil
	}

	client := hyperapi.NewClient()
	client.Auth(secret, account) // Note: Hyperliquid uses wallet address as key

	ex := &Exchange{
		client: client,
	}
	ex.UseFutures()

	ctx := context.Background()
	if err := ex.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize exchange: %v", err)
	}

	stream := NewStream(client, ex)
	return stream
}

func TestStream_Integration_Book(t *testing.T) {
	stream := getTestClientOrSkip(t)

	stream.Subscribe(types.BookChannel, "BTCUSDC", types.SubscribeOptions{})
	stream.SetPublicOnly()

	bookReceived := make(chan bool, 1)
	stream.OnBookSnapshot(func(book types.SliceOrderBook) {
		select {
		case bookReceived <- true:
		default:
		}
	})

	err := stream.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = stream.Close()
	})

	select {
	case <-bookReceived:
		t.Log("Book snapshot received successfully")
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for book snapshot")
	}
}

func TestStream_Integration_Trades(t *testing.T) {
	stream := getTestClientOrSkip(t)

	stream.Subscribe(types.MarketTradeChannel, "BTCUSDC", types.SubscribeOptions{})
	stream.SetPublicOnly()

	tradeReceived := make(chan bool, 1)
	stream.OnMarketTrade(func(trade types.Trade) {
		t.Logf("Trade: %s %s %s @ %s", trade.Symbol, trade.Side, trade.Quantity, trade.Price)
		select {
		case tradeReceived <- true:
		default:
		}
	})

	err := stream.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = stream.Close()
	})

	select {
	case <-tradeReceived:
		t.Log("Trade received successfully")
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for trade")
	}
}

func TestStream_Integration_KLine(t *testing.T) {
	stream := getTestClientOrSkip(t)

	stream.Subscribe(types.KLineChannel, "BTCUSDC", types.SubscribeOptions{
		Interval: types.Interval1m,
	})
	stream.SetPublicOnly()

	klineReceived := make(chan bool, 1)
	stream.OnKLine(func(kline types.KLine) {
		t.Logf("KLine: %s %s O:%s C:%s H:%s L:%s V:%s",
			kline.Symbol, kline.Interval, kline.Open, kline.Close, kline.High, kline.Low, kline.Volume)
		select {
		case klineReceived <- true:
		default:
		}
	})

	err := stream.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = stream.Close()
	})

	select {
	case <-klineReceived:
		t.Log("KLine received successfully")
	case <-time.After(90 * time.Second): // KLine might take longer
		t.Fatal("Timeout waiting for kline")
	}
}

func TestStream_Integration_PrivateChannels(t *testing.T) {
	t.Skip("Integration test - run manually with real account")
	stream := getTestClientOrSkip(t)

	authReceived := make(chan bool, 1)
	stream.OnAuth(func() {
		t.Log("Authenticated successfully")
		select {
		case authReceived <- true:
		default:
		}
	})

	orderReceived := make(chan bool, 1)
	stream.OnOrderUpdate(func(order types.Order) {
		t.Logf("Order update: %d %s %s %s @ %s",
			order.OrderID, order.Symbol, order.Status, order.Side, order.Price)
		select {
		case orderReceived <- true:
		default:
		}
	})

	tradeReceived := make(chan bool, 1)
	stream.OnTradeUpdate(func(trade types.Trade) {
		t.Logf("Trade update: %d %s %s %s @ %s",
			trade.OrderID, trade.Symbol, trade.Side, trade.Quantity, trade.Price)
		select {
		case tradeReceived <- true:
		default:
		}
	})

	positionReceived := make(chan bool, 1)
	stream.OnFuturesPositionUpdate(func(positions types.FuturesPositionMap) {
		t.Logf("Position update: %d positions", len(positions))
		for key, pos := range positions {
			t.Logf("  %s: %s %s", key, pos.PositionSide, pos.Base)
		}
		select {
		case positionReceived <- true:
		default:
		}
	})

	err := stream.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = stream.Close()
	})

	select {
	case <-authReceived:
		t.Log("Auth event received")
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for auth")
	}

	// Wait for any private channel events (optional)
	select {
	case <-orderReceived:
		t.Log("Order update received")
	case <-tradeReceived:
		t.Log("Trade update received")
	case <-positionReceived:
		t.Log("Position update received")
	case <-time.After(60 * time.Second):
		t.Log("No private channel events received (this is normal if no activity)")
	}
}
