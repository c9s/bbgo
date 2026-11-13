package backpack

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/backpack/backpackapi"
	"github.com/c9s/bbgo/pkg/types"
)

// clearSymbolMaps resets the package level symbol maps so that tests do not leak state into
// each other.
func clearSymbolMaps(t *testing.T) {
	t.Helper()

	for _, m := range []*sync.Map{&spotSymbolSyncMap, &perpSymbolSyncMap} {
		m.Range(func(k, _ any) bool {
			m.Delete(k)
			return true
		})
	}
}

func TestSplitGlobalSymbol(t *testing.T) {
	quotes := sortQuoteCurrenciesByLength(defaultQuoteCurrencies)

	cases := []struct {
		global string
		base   string
		quote  string
		ok     bool
	}{
		// the four character quotes must win over the three character ones, otherwise
		// BTCUSDT would be split into BTC + USD with a stray T
		{"BTCUSDT", "BTC", "USDT", true},
		{"BTCUSDC", "BTC", "USDC", true},
		{"BTCUSD", "BTC", "USD", true},
		{"SOMEBUSD", "SOME", "BUSD", true},
		{"SOMETUSD", "SOME", "TUSD", true},

		// a stablecoin can be the base as well
		{"USDTUSDC", "USDT", "USDC", true},
		{"USDCUSDT", "USDC", "USDT", true},

		// non fiat quotes
		{"ETHBTC", "ETH", "BTC", true},
		{"SOLETH", "SOL", "ETH", true},

		// no known quote suffix
		{"SOMETHING", "", "", false},
		// the base part may not be empty
		{"USDT", "", "", false},
		{"", "", "", false},
	}

	for _, c := range cases {
		t.Run(c.global, func(t *testing.T) {
			base, quote, ok := splitGlobalSymbol(c.global, quotes)
			assert.Equal(t, c.ok, ok)
			assert.Equal(t, c.base, base)
			assert.Equal(t, c.quote, quote)
		})
	}
}

func TestSortQuoteCurrenciesByLength(t *testing.T) {
	sorted := sortQuoteCurrenciesByLength(defaultQuoteCurrencies)

	assert.Len(t, sorted, len(defaultQuoteCurrencies))
	for i := 1; i < len(sorted); i++ {
		assert.GreaterOrEqual(t, len(sorted[i-1]), len(sorted[i]),
			"the candidates must be ordered by descending length")
	}

	// the input must not be mutated
	assert.Equal(t, "USDT", defaultQuoteCurrencies[0])
}

func TestToGlobalSymbol(t *testing.T) {
	assert.Equal(t, "SOLUSDC", toGlobalSymbol("SOL_USDC"))
	assert.Equal(t, "USDTUSDC", toGlobalSymbol("USDT_USDC"))
	assert.Equal(t, "BTCUSDC", toGlobalSymbol("BTC_USDC"))

	// the perp suffix is dropped, so a perp collapses onto the spot symbol; the market type
	// tells them apart on the way back
	assert.Equal(t, "SOLUSDC", toGlobalSymbol("SOL_USDC_PERP"))
}

func TestToLocalSymbol(t *testing.T) {
	clearSymbolMaps(t)

	t.Run("fallback without the symbol map", func(t *testing.T) {
		assert.Equal(t, "SOL_USDC", toLocalSymbol("SOLUSDC"))
		assert.Equal(t, "BTC_USDC", toLocalSymbol("BTCUSDC"))

		// the base is a stablecoin itself
		assert.Equal(t, "USDT_USDC", toLocalSymbol("USDTUSDC"))

		// the fallback is not tied to USDC
		assert.Equal(t, "BTC_USDT", toLocalSymbol("BTCUSDT"))
		assert.Equal(t, "BTC_USD", toLocalSymbol("BTCUSD"))
	})

	t.Run("fallback for perp", func(t *testing.T) {
		assert.Equal(t, "SOL_USDC_PERP", toLocalSymbol("SOLUSDC", backpackapi.MarketTypePerp))
	})

	t.Run("unknown quote is passed through", func(t *testing.T) {
		assert.Equal(t, "SOMETHING", toLocalSymbol("SOMETHING"))
	})

	t.Run("the symbol map wins over the fallback", func(t *testing.T) {
		clearSymbolMaps(t)
		defer clearSymbolMaps(t)

		syncLocalSymbolMap(types.MarketMap{
			"WEIRDUSDC": types.Market{Symbol: "WEIRDUSDC", LocalSymbol: "WEIRD_USDC"},
		}, backpackapi.MarketTypeSpot)

		assert.Equal(t, "WEIRD_USDC", toLocalSymbol("WEIRDUSDC"))
	})
}

func TestSyncLocalSymbolMap(t *testing.T) {
	clearSymbolMaps(t)
	defer clearSymbolMaps(t)

	syncLocalSymbolMap(types.MarketMap{
		"AAAUSDC": types.Market{Symbol: "AAAUSDC", LocalSymbol: "AAA_USDC"},
		"BBBUSDC": types.Market{Symbol: "BBBUSDC", LocalSymbol: "BBB_USDC"},
	}, backpackapi.MarketTypeSpot)

	assert.Equal(t, "AAA_USDC", toLocalSymbol("AAAUSDC"))
	assert.Equal(t, "BBB_USDC", toLocalSymbol("BBBUSDC"))

	// BBB is delisted, AAA stays and CCC is new
	syncLocalSymbolMap(types.MarketMap{
		"AAAUSDC": types.Market{Symbol: "AAAUSDC", LocalSymbol: "AAA_USDC"},
		"CCCUSDC": types.Market{Symbol: "CCCUSDC", LocalSymbol: "CCC_USDC"},
	}, backpackapi.MarketTypeSpot)

	_, stillThere := spotSymbolSyncMap.Load("BBBUSDC")
	assert.False(t, stillThere, "a delisted symbol should be removed from the map")

	assert.Equal(t, "CCC_USDC", toLocalSymbol("CCCUSDC"))
}

// TestSymbolRoundTrip checks the symbol conversion against the real Backpack market list.
func TestSymbolRoundTrip(t *testing.T) {
	clearSymbolMaps(t)
	defer clearSymbolMaps(t)

	// a representative sample of the live spot markets, including the stablecoin pair and the
	// numeric-prefixed tickers
	localSymbols := []string{
		"SOL_USDC", "BTC_USDC", "ETH_USDC", "USDT_USDC", "PYTH_USDC",
		"BONK_USDC", "0G_USDC", "2Z_USDC", "FARTCOIN_USDC", "WBTC_USDC",
	}

	for _, local := range localSymbols {
		t.Run(local, func(t *testing.T) {
			global := toGlobalSymbol(local)
			assert.NotContains(t, global, "_")

			// without the map the fallback has to reconstruct the local symbol
			assert.Equal(t, local, toLocalSymbol(global),
				"the fallback should round-trip %s", local)
		})
	}

	t.Run("perp round trip", func(t *testing.T) {
		local := "SOL_USDC_PERP"
		global := toGlobalSymbol(local)
		assert.Equal(t, "SOLUSDC", global)
		assert.Equal(t, local, toLocalSymbol(global, backpackapi.MarketTypePerp))
	})
}

func TestToGlobalSide(t *testing.T) {
	// Backpack uses Bid/Ask rather than BUY/SELL
	assert.Equal(t, types.SideTypeBuy, toGlobalSide(backpackapi.SideBid))
	assert.Equal(t, types.SideTypeSell, toGlobalSide(backpackapi.SideAsk))

	side, err := toLocalSide(types.SideTypeBuy)
	assert.NoError(t, err)
	assert.Equal(t, backpackapi.SideBid, side)

	side, err = toLocalSide(types.SideTypeSell)
	assert.NoError(t, err)
	assert.Equal(t, backpackapi.SideAsk, side)

	_, err = toLocalSide("NOPE")
	assert.Error(t, err)
}

func TestOrderTypeConversion(t *testing.T) {
	t.Run("post only maps to limit maker", func(t *testing.T) {
		assert.Equal(t, types.OrderTypeLimit, toGlobalOrderType(backpackapi.OrderTypeLimit, false))
		assert.Equal(t, types.OrderTypeLimitMaker, toGlobalOrderType(backpackapi.OrderTypeLimit, true))
		assert.Equal(t, types.OrderTypeMarket, toGlobalOrderType(backpackapi.OrderTypeMarket, false))
	})

	t.Run("limit maker becomes limit plus post only", func(t *testing.T) {
		localType, postOnly, err := toLocalOrderType(types.OrderTypeLimitMaker)
		assert.NoError(t, err)
		assert.Equal(t, backpackapi.OrderTypeLimit, localType)
		assert.True(t, postOnly)

		localType, postOnly, err = toLocalOrderType(types.OrderTypeLimit)
		assert.NoError(t, err)
		assert.Equal(t, backpackapi.OrderTypeLimit, localType)
		assert.False(t, postOnly)

		localType, postOnly, err = toLocalOrderType(types.OrderTypeMarket)
		assert.NoError(t, err)
		assert.Equal(t, backpackapi.OrderTypeMarket, localType)
		assert.False(t, postOnly)

		_, _, err = toLocalOrderType(types.OrderTypeStopMarket)
		assert.Error(t, err)
	})
}

func TestToGlobalOrderStatus(t *testing.T) {
	cases := map[backpackapi.OrderStatus]types.OrderStatus{
		backpackapi.OrderStatusNew:             types.OrderStatusNew,
		backpackapi.OrderStatusPartiallyFilled: types.OrderStatusPartiallyFilled,
		backpackapi.OrderStatusFilled:          types.OrderStatusFilled,
		backpackapi.OrderStatusCancelled:       types.OrderStatusCanceled,
		backpackapi.OrderStatusExpired:         types.OrderStatusExpired,
		backpackapi.OrderStatusTriggerPending:  types.OrderStatusTriggering,
		backpackapi.OrderStatusTriggerFailed:   types.OrderStatusRejected,
	}

	for local, global := range cases {
		got, err := toGlobalOrderStatus(local)
		if assert.NoError(t, err) {
			assert.Equal(t, global, got)
		}
	}

	// an unknown status is an error rather than a silent pass-through
	_, err := toGlobalOrderStatus("Whatever")
	assert.Error(t, err)
}

func TestToGlobalOrderID(t *testing.T) {
	// Backpack ids are decimal strings in practice, keep the real id
	assert.Equal(t, uint64(59669287267), toGlobalOrderID("59669287267"))

	// a non numeric id is hashed instead of failing
	hashed := toGlobalOrderID("42f4f364-82e1-49d3-ad1d-cd8cf9aa308d")
	assert.NotZero(t, hashed)
	assert.Equal(t, hashed, toGlobalOrderID("42f4f364-82e1-49d3-ad1d-cd8cf9aa308d"),
		"the hash must be stable")
}

// TestToGlobalOrder decodes a response captured from the live API.
func TestToGlobalOrder(t *testing.T) {
	payload := `{
      "clientId": null, "createdAt": 1788285157776,
      "executedQuantity": "0", "executedQuoteQuantity": "0",
      "id": "59669039887", "orderType": "Limit", "postOnly": true,
      "price": "1.0077", "quantity": "20", "reduceOnly": null,
      "selfTradePrevention": "RejectTaker", "side": "Ask", "status": "New",
      "symbol": "USDT_USDC", "timeInForce": "GTC", "triggeredAt": null
    }`

	var o backpackapi.Order
	if !assert.NoError(t, json.Unmarshal([]byte(payload), &o)) {
		return
	}

	order, err := toGlobalOrder(&o)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, types.ExchangeBackpack, order.Exchange)
	assert.Equal(t, "USDTUSDC", order.Symbol)
	assert.Equal(t, types.SideTypeSell, order.Side)
	assert.Equal(t, types.OrderTypeLimitMaker, order.Type, "postOnly should map to limit maker")
	assert.Equal(t, types.OrderStatusNew, order.Status)
	assert.Equal(t, "New", order.OriginalStatus)
	assert.Equal(t, types.TimeInForceGTC, order.TimeInForce)
	assert.Equal(t, "1.0077", order.Price.String())
	assert.Equal(t, "20", order.Quantity.String())
	assert.True(t, order.ExecutedQuantity.IsZero())
	assert.True(t, order.IsWorking)
	assert.Equal(t, uint64(59669039887), order.OrderID)
	assert.Equal(t, "59669039887", order.UUID)
	assert.Empty(t, order.ClientOrderID, "a null clientId should not become \"0\"")
	assert.False(t, order.CreationTime.Time().IsZero())
}

// TestOrderHistoryEntryToGlobalOrder decodes a response captured from the live API.
// The history endpoint encodes createdAt as a naive datetime rather than a ms timestamp.
func TestOrderHistoryEntryToGlobalOrder(t *testing.T) {
	payload := `{
      "clientId": null, "createdAt": "2026-09-01T17:54:04.094",
      "executedQuantity": "0", "executedQuoteQuantity": "0", "expiryReason": null,
      "id": "59669287267", "orderType": "Limit", "postOnly": true,
      "price": "1.0077", "quantity": "20", "quoteQuantity": "20.154",
      "selfTradePrevention": "RejectTaker", "side": "Ask",
      "status": "Cancelled", "symbol": "USDT_USDC", "systemOrderType": null,
      "timeInForce": "GTC"
    }`

	var e backpackapi.OrderHistoryEntry
	if !assert.NoError(t, json.Unmarshal([]byte(payload), &e)) {
		return
	}

	order, err := orderHistoryEntryToGlobalOrder(&e)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "USDTUSDC", order.Symbol)
	assert.Equal(t, types.OrderStatusCanceled, order.Status)
	assert.False(t, order.IsWorking)
	assert.Equal(t, uint64(59669287267), order.OrderID)
	assert.Equal(t,
		time.Date(2026, time.September, 1, 17, 54, 4, 94000000, time.UTC),
		order.CreationTime.Time().UTC())
}

func TestToGlobalTrade(t *testing.T) {
	payload := `{"fee":"0.01","feeSymbol":"USDC","isMaker":true,"orderId":"59669287267",
      "price":"1.0077","quantity":"20","side":"Ask","symbol":"USDT_USDC",
      "timestamp":"2026-09-01T17:54:04.094","tradeId":381731330}`

	var fill backpackapi.OrderFill
	if !assert.NoError(t, json.Unmarshal([]byte(payload), &fill)) {
		return
	}

	trade := toGlobalTrade(&fill)
	assert.Equal(t, types.ExchangeBackpack, trade.Exchange)
	assert.Equal(t, uint64(381731330), trade.ID)
	assert.Equal(t, uint64(59669287267), trade.OrderID)
	assert.Equal(t, "59669287267", trade.OrderUUID)
	assert.Equal(t, "USDTUSDC", trade.Symbol)
	assert.Equal(t, types.SideTypeSell, trade.Side)
	assert.False(t, trade.IsBuyer)
	assert.True(t, trade.IsMaker)
	assert.Equal(t, "20.154", trade.QuoteQuantity.String())
	assert.Equal(t, "USDC", trade.FeeCurrency)
}

func TestToGlobalBalance(t *testing.T) {
	payload := `{"USDT":{"available":"30","locked":"0","staked":"0"},
                "SOL":{"available":"1.5","locked":"0.5","staked":"2"}}`

	var balances backpackapi.BalanceMap
	if !assert.NoError(t, json.Unmarshal([]byte(payload), &balances)) {
		return
	}

	balanceMap := toGlobalBalance(balances)
	assert.Len(t, balanceMap, 2)

	assert.Equal(t, "30", balanceMap["USDT"].Available.String())
	assert.True(t, balanceMap["USDT"].Locked.IsZero())

	// staked funds are not tradable, they are folded into Locked
	assert.Equal(t, "1.5", balanceMap["SOL"].Available.String())
	assert.Equal(t, "2.5", balanceMap["SOL"].Locked.String())
	assert.Equal(t, "4", balanceMap["SOL"].NetAsset.String())
}

func TestToGlobalKLine(t *testing.T) {
	payload := `{"close":"101.1","end":"2026-09-01 17:24:00","high":"101.13","low":"101.1",
      "open":"101.13","quoteVolume":"44.4906","start":"2026-09-01 17:23:00",
      "trades":"2","volume":"0.44"}`

	var k backpackapi.Kline
	if !assert.NoError(t, json.Unmarshal([]byte(payload), &k)) {
		return
	}

	kline := toGlobalKLine("SOLUSDC", types.Interval1m, &k)
	assert.Equal(t, types.ExchangeBackpack, kline.Exchange)
	assert.Equal(t, "SOLUSDC", kline.Symbol)
	assert.Equal(t, types.Interval1m, kline.Interval)
	assert.Equal(t, "101.13", kline.Open.String())
	assert.Equal(t, "101.1", kline.Close.String())
	assert.Equal(t, uint64(2), kline.NumberOfTrades)

	// EndTime follows the binance rule: one millisecond before the next start time
	assert.Equal(t,
		kline.StartTime.Time().Add(time.Minute-time.Millisecond),
		kline.EndTime.Time())
}

func TestToGlobalMarket(t *testing.T) {
	payload := `{
      "baseSymbol":"SOL","createdAt":"2025-01-21T06:34:54.691858",
      "filters":{
        "price":{"minPrice":"0.01","tickSize":"0.01","maxPrice":null},
        "quantity":{"maxQuantity":null,"minQuantity":"0.01","stepSize":"0.01"}},
      "marketType":"SPOT","orderBookState":"Open","quoteSymbol":"USDC",
      "symbol":"SOL_USDC","visible":true
    }`

	var m backpackapi.Market
	if !assert.NoError(t, json.Unmarshal([]byte(payload), &m)) {
		return
	}

	market := toGlobalMarket(&m)
	assert.Equal(t, types.ExchangeBackpack, market.Exchange)
	assert.Equal(t, "SOLUSDC", market.Symbol)
	assert.Equal(t, "SOL_USDC", market.LocalSymbol, "LocalSymbol feeds the symbol map")
	assert.Equal(t, "SOL", market.BaseCurrency)
	assert.Equal(t, "USDC", market.QuoteCurrency)
	assert.Equal(t, "0.01", market.TickSize.String())
	assert.Equal(t, "0.01", market.StepSize.String())

	// the precision is derived from the tick/step size, not from math.Log10
	assert.Equal(t, 2, market.PricePrecision)
	assert.Equal(t, 2, market.VolumePrecision)
}

func TestToLocalInterval(t *testing.T) {
	interval, err := toLocalInterval(types.Interval1m)
	assert.NoError(t, err)
	assert.Equal(t, backpackapi.KlineInterval1m, interval)

	// bbgo spells it 1mo, Backpack spells it 1month
	interval, err = toLocalInterval(types.Interval1mo)
	assert.NoError(t, err)
	assert.Equal(t, backpackapi.KlineInterval1month, interval)

	// bbgo has 2w, Backpack does not; this must fail loudly rather than silently pick another
	_, err = toLocalInterval(types.Interval2w)
	assert.Error(t, err)
}

func TestParseClientOrderID(t *testing.T) {
	id, err := parseClientOrderID("12345")
	assert.NoError(t, err)
	assert.Equal(t, uint32(12345), id)

	_, err = parseClientOrderID("not-a-number")
	assert.Error(t, err)

	// the client order id has to fit in a uint32
	_, err = parseClientOrderID("4294967296")
	assert.Error(t, err)
}

// TestIsLocalSymbolOfMarketType guards against perp markets collapsing onto their spot symbol.
//
// toGlobalSymbol strips the perp suffix, so SOL_USDC and SOL_USDC_PERP both become SOLUSDC.
// Endpoints without a market type filter return both, and without this check one would
// silently overwrite the other in the resulting map.
func TestIsLocalSymbolOfMarketType(t *testing.T) {
	assert.True(t, isLocalSymbolOfMarketType("SOL_USDC", backpackapi.MarketTypeSpot))
	assert.False(t, isLocalSymbolOfMarketType("SOL_USDC_PERP", backpackapi.MarketTypeSpot))

	assert.True(t, isLocalSymbolOfMarketType("SOL_USDC_PERP", backpackapi.MarketTypePerp))
	assert.False(t, isLocalSymbolOfMarketType("SOL_USDC", backpackapi.MarketTypePerp))
}

// TestToGlobalOrderBookOrdering guards the order book side ordering.
//
// Backpack sends both sides in ascending price order, but types.SliceOrderBook.BestBid reads
// Bids[0], so the bids have to be flipped. Getting this wrong produces a book whose best bid is
// the *worst* price on the book, which still looks plausible in a log.
func TestToGlobalOrderBookOrdering(t *testing.T) {
	payload := `{
      "asks": [["101.01","1"],["101.02","2"],["101.03","3"]],
      "bids": [["100.96","1"],["100.97","2"],["100.98","3"]],
      "lastUpdateId": "4324419962",
      "timestamp": 1788283696669335
    }`

	var depth backpackapi.Depth
	if !assert.NoError(t, json.Unmarshal([]byte(payload), &depth)) {
		return
	}

	book := toGlobalOrderBook("SOLUSDC", &depth)

	assert.Equal(t, "SOLUSDC", book.Symbol)
	assert.Equal(t, int64(4324419962), book.LastUpdateId)

	bestBid, ok := book.BestBid()
	if assert.True(t, ok) {
		assert.Equal(t, "100.98", bestBid.Price.String(), "the best bid is the highest bid")
	}

	bestAsk, ok := book.BestAsk()
	if assert.True(t, ok) {
		assert.Equal(t, "101.01", bestAsk.Price.String(), "the best ask is the lowest ask")
	}

	assert.True(t, bestBid.Price.Compare(bestAsk.Price) < 0, "the book must not be crossed")

	// the bids descend and the asks ascend
	for i := 1; i < len(book.Bids); i++ {
		assert.True(t, book.Bids[i-1].Price.Compare(book.Bids[i].Price) > 0)
	}

	for i := 1; i < len(book.Asks); i++ {
		assert.True(t, book.Asks[i-1].Price.Compare(book.Asks[i].Price) < 0)
	}
}

// TestDepthEventToGlobalOrderBook checks the same ordering for an incremental update, and that
// a removed level survives as a zero quantity.
func TestDepthEventToGlobalOrderBook(t *testing.T) {
	payload := `{"stream":"depth.SOL_USDC","data":{
      "E":1788367430247494,"T":1788367430246081,"U":4330360995,
      "a":[["98.97","22.39"],["98.99","0"]],"b":[["98.90","1"],["98.95","2"]],
      "e":"depth","s":"SOL_USDC","u":4330360996}}`

	event, err := backpackapi.ParseWebsocketMessage([]byte(payload))
	if !assert.NoError(t, err) {
		return
	}

	depthEvent, ok := event.(*backpackapi.DepthEvent)
	if !assert.True(t, ok) {
		return
	}

	book := depthEventToGlobalOrderBook(depthEvent)
	assert.Equal(t, "SOLUSDC", book.Symbol)
	assert.Equal(t, int64(4330360996), book.LastUpdateId)

	assert.Equal(t, "98.95", book.Bids[0].Price.String(), "the bids descend")
	assert.Equal(t, "98.97", book.Asks[0].Price.String(), "the asks ascend")

	// a removed level arrives as a zero quantity and must not be dropped
	assert.Len(t, book.Asks, 2)
	assert.True(t, book.Asks[1].Volume.IsZero())
}
