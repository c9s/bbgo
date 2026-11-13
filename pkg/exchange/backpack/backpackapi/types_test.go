package backpackapi

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestMicrosecondTimestamp(t *testing.T) {
	t.Run("number", func(t *testing.T) {
		var ts MicrosecondTimestamp
		if assert.NoError(t, json.Unmarshal([]byte("1788283696669335"), &ts)) {
			assert.Equal(t, int64(1788283696669335), ts.Time().UnixMicro())
		}
	})

	t.Run("string", func(t *testing.T) {
		var ts MicrosecondTimestamp
		if assert.NoError(t, json.Unmarshal([]byte(`"1788283696669335"`), &ts)) {
			assert.Equal(t, int64(1788283696669335), ts.Time().UnixMicro())
		}
	})

	t.Run("null", func(t *testing.T) {
		var ts MicrosecondTimestamp
		if assert.NoError(t, json.Unmarshal([]byte("null"), &ts)) {
			assert.True(t, ts.Time().IsZero())
		}
	})
}

func TestKlineTime(t *testing.T) {
	t.Run("space separated utc datetime", func(t *testing.T) {
		var kt KlineTime
		if assert.NoError(t, json.Unmarshal([]byte(`"2026-09-01 17:23:00"`), &kt)) {
			assert.Equal(t,
				time.Date(2026, time.September, 1, 17, 23, 0, 0, time.UTC),
				kt.Time().UTC())
		}
	})

	t.Run("rfc3339 fallback", func(t *testing.T) {
		var kt KlineTime
		if assert.NoError(t, json.Unmarshal([]byte(`"2026-09-01T17:23:00Z"`), &kt)) {
			assert.Equal(t,
				time.Date(2026, time.September, 1, 17, 23, 0, 0, time.UTC),
				kt.Time().UTC())
		}
	})

	t.Run("invalid", func(t *testing.T) {
		var kt KlineTime
		assert.Error(t, json.Unmarshal([]byte(`"not a time"`), &kt))
	})
}

// TestNaiveDatetimeDecoding pins down the assumption that types.Time can decode the naive
// datetime strings used by the order and fill history endpoints.
func TestNaiveDatetimeDecoding(t *testing.T) {
	var ts types.Time
	if assert.NoError(t, json.Unmarshal([]byte(`"2025-01-21T06:34:54.691858"`), &ts)) {
		assert.Equal(t,
			time.Date(2025, time.January, 21, 6, 34, 54, 691858000, time.UTC),
			ts.Time().UTC())
	}
}

func TestPriceLevel(t *testing.T) {
	t.Run("pair", func(t *testing.T) {
		var level PriceLevel
		if assert.NoError(t, json.Unmarshal([]byte(`["101.01","191.66"]`), &level)) {
			assert.Equal(t, "101.01", level.Price.String())
			assert.Equal(t, "191.66", level.Quantity.String())
		}
	})

	t.Run("wrong arity", func(t *testing.T) {
		var level PriceLevel
		assert.ErrorContains(t, json.Unmarshal([]byte(`["101.01"]`), &level), "price, quantity")
	})
}

// TestDepthDecoding decodes a response captured from the live api.
func TestDepthDecoding(t *testing.T) {
	payload := `{
      "asks": [["101.01","191.66"],["101.02","228.03"]],
      "bids": [["100.96","188.09"],["100.97","36.76"]],
      "lastUpdateId": "4324419962",
      "timestamp": 1788283696669335
    }`

	var depth Depth
	if assert.NoError(t, json.Unmarshal([]byte(payload), &depth)) {
		assert.Len(t, depth.Asks, 2)
		assert.Len(t, depth.Bids, 2)
		assert.Equal(t, "101.01", depth.Asks[0].Price.String())
		assert.Equal(t, int64(4324419962), int64(depth.LastUpdateId))
		assert.Equal(t, int64(1788283696669335), depth.Timestamp.Time().UnixMicro())
	}
}

// TestKlineDecoding decodes a response captured from the live api.
func TestKlineDecoding(t *testing.T) {
	payload := `{
      "close":"101.1","end":"2026-09-01 17:24:00","high":"101.13","low":"101.1",
      "open":"101.13","quoteVolume":"44.4906","start":"2026-09-01 17:23:00",
      "trades":"2","volume":"0.44"
    }`

	var kline Kline
	if assert.NoError(t, json.Unmarshal([]byte(payload), &kline)) {
		assert.Equal(t, "101.13", kline.Open.String())
		assert.Equal(t, "101.1", kline.Close.String())
		assert.Equal(t, int64(2), int64(kline.Trades))
		assert.Equal(t, int64(60), int64(kline.End.Time().Sub(kline.Start.Time())/time.Second))
	}

	t.Run("empty interval has null prices", func(t *testing.T) {
		var kline Kline
		payload := `{"close":null,"end":"2026-09-01 17:24:00","high":null,"low":null,
          "open":null,"quoteVolume":"0","start":"2026-09-01 17:23:00","trades":"0","volume":"0"}`

		if assert.NoError(t, json.Unmarshal([]byte(payload), &kline)) {
			assert.True(t, kline.Open.IsZero())
			assert.True(t, kline.Close.IsZero())
		}
	})
}

// TestMarketDecoding decodes a spot and a perp market captured from the live api, covering the
// nullable futures-only fields.
func TestMarketDecoding(t *testing.T) {
	payload := `{
      "baseSymbol":"SOL","createdAt":"2025-01-21T06:34:54.691858",
      "filters":{
        "price":{"minPrice":"0.01","tickSize":"0.01","maxPrice":null,"maxMultiplier":"2",
                 "minMultiplier":"0.25",
                 "meanMarkPriceBand":{"maxMultiplier":"1.03","minMultiplier":"0.97"},
                 "meanPremiumBand":null,"discoveryBoundBand":null},
        "quantity":{"maxQuantity":null,"minQuantity":"0.01","stepSize":"0.01"}},
      "fundingInterval":null,"fundingRateLowerBound":null,"fundingRateUpperBound":null,
      "imfFunction":null,"mmfFunction":null,"marketType":"SPOT","openInterestLimit":"0",
      "orderBookState":"Open","positionLimitWeight":null,"quoteSymbol":"USDC",
      "rwaMarketType":null,"symbol":"SOL_USDC","visible":true
    }`

	var market Market
	if assert.NoError(t, json.Unmarshal([]byte(payload), &market)) {
		assert.Equal(t, "SOL_USDC", market.Symbol)
		assert.Equal(t, MarketTypeSpot, market.MarketType)
		assert.Equal(t, OrderBookStateOpen, market.OrderBookState)
		assert.Equal(t, "0.01", market.Filters.Price.TickSize.String())
		assert.Equal(t, "0.01", market.Filters.Quantity.StepSize.String())
		assert.True(t, market.Filters.Price.MaxPrice.IsZero(), "a null maxPrice decodes to zero")
		assert.Nil(t, market.ImfFunction)
		assert.Equal(t, uint64(0), market.FundingInterval)
		assert.False(t, market.CreatedAt.Time().IsZero())
	}

	t.Run("perp", func(t *testing.T) {
		payload := `{
          "baseSymbol":"SOL","createdAt":"2025-01-21T06:34:54.691858",
          "filters":{"price":{"minPrice":"0.01","tickSize":"0.01"},
                     "quantity":{"minQuantity":"0.01","stepSize":"0.01"}},
          "marketType":"PERP","fundingInterval":3600000,
          "fundingRateLowerBound":"-100","fundingRateUpperBound":"100",
          "imfFunction":{"base":"0.02","factor":"0.000075","type":"sqrt"},
          "openInterestLimit":"4000000","orderBookState":"Open",
          "positionLimitWeight":"1","quoteSymbol":"USDC","symbol":"SOL_USDC_PERP","visible":true
        }`

		var market Market
		if assert.NoError(t, json.Unmarshal([]byte(payload), &market)) {
			assert.Equal(t, MarketTypePerp, market.MarketType)
			assert.Equal(t, uint64(3600000), market.FundingInterval)
			if assert.NotNil(t, market.ImfFunction) {
				assert.Equal(t, "sqrt", market.ImfFunction.Type)
				assert.Equal(t, "0.02", market.ImfFunction.Base.String())
			}
		}
	})
}

func TestTickerDecoding(t *testing.T) {
	payload := `{"firstPrice":"102.67","high":"104.96","lastPrice":"101.04","low":"100.57",
      "priceChange":"-1.63","priceChangePercent":"-0.015876","quoteVolume":"1175485.13",
      "symbol":"SOL_USDC","trades":"3858","volume":"11469.17"}`

	var ticker Ticker
	if assert.NoError(t, json.Unmarshal([]byte(payload), &ticker)) {
		assert.Equal(t, "SOL_USDC", ticker.Symbol)
		assert.Equal(t, "101.04", ticker.LastPrice.String())
		assert.Equal(t, int64(3858), int64(ticker.Trades))
		assert.True(t, ticker.PriceChange.Sign() < 0)
	}
}

func TestBalanceMapDecoding(t *testing.T) {
	payload := `{"SOL":{"available":"1.5","locked":"0.5","staked":"0"},
                "USDC":{"available":"100","locked":"0","staked":"0"}}`

	var balances BalanceMap
	if assert.NoError(t, json.Unmarshal([]byte(payload), &balances)) {
		assert.Len(t, balances, 2)
		assert.Equal(t, "2", balances["SOL"].Total().String())
		assert.Equal(t, "100", balances["USDC"].Available.String())
	}
}

func TestBatchOrderResultDecoding(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		payload := `{"operation":"Ok","id":"114","symbol":"SOL_USDC","side":"Bid",
          "status":"New","orderType":"Limit","price":"50","quantity":"1",
          "executedQuantity":"0","executedQuoteQuantity":"0","timeInForce":"GTC",
          "selfTradePrevention":"RejectTaker","createdAt":1788283696669}`

		var result BatchOrderResult
		if assert.NoError(t, json.Unmarshal([]byte(payload), &result)) {
			assert.True(t, result.IsOk())
			if assert.NotNil(t, result.Order) {
				assert.Equal(t, "114", result.Order.Id)
				assert.Equal(t, SideBid, result.Order.Side)
			}
		}
	})

	t.Run("err", func(t *testing.T) {
		payload := `{"operation":"Err","code":"INSUFFICIENT_FUNDS","message":"Insufficient funds"}`

		var result BatchOrderResult
		if assert.NoError(t, json.Unmarshal([]byte(payload), &result)) {
			assert.False(t, result.IsOk())
			if assert.NotNil(t, result.Error) {
				assert.Equal(t, ApiErrorCodeInsufficientFunds, result.Error.Code)
			}
		}
	})
}

func TestOrderFillDecoding(t *testing.T) {
	payload := `{"fee":"0.01","feeSymbol":"USDC","isMaker":true,"orderId":"114",
      "price":"101.04","quantity":"0.5","side":"Bid","symbol":"SOL_USDC",
      "timestamp":"2025-06-20T06:11:10.123456","tradeId":381731330,"clientId":"42"}`

	var fill OrderFill
	if assert.NoError(t, json.Unmarshal([]byte(payload), &fill)) {
		assert.Equal(t, "114", fill.OrderId)
		assert.True(t, fill.IsMaker)
		assert.Equal(t, int64(381731330), fill.TradeId)
		// clientId is a string on this endpoint, unlike the uint32 on the order endpoints
		assert.Equal(t, "42", fill.ClientId)
		assert.False(t, fill.Timestamp.Time().IsZero())
	}
}
