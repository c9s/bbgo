package okex

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/strint"
)

func Test_orderDetailToGlobal(t *testing.T) {
	var (
		assert = assert.New(t)

		orderId = 665576973905014786
		// {"accFillSz":"0","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"","cTime":"1704957916401","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"0","feeCcy":"USDT","fillPx":"","fillSz":"0","fillTime":"","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665576973905014786","ordType":"limit","pnl":"0","posSide":"net","px":"48174.5","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"BTC","reduceOnly":"false","side":"sell","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"live","stpId":"","stpMode":"","sz":"0.00001","tag":"","tdMode":"cash","tgtCcy":"","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"","uTime":"1704957916401"}
		openOrder = &okexapi.OrderDetail{
			AccumulatedFillSize: fixedpoint.NewFromFloat(0),
			AvgPrice:            fixedpoint.NewFromFloat(0),
			CreatedTime:         types.NewMillisecondTimestampFromInt(1704957916401),
			Category:            "normal",
			Currency:            "BTC",
			ClientOrderId:       "",
			Fee:                 fixedpoint.Zero,
			FeeCurrency:         "USDT",
			FillTime:            types.NewMillisecondTimestampFromInt(0),
			InstrumentID:        "BTC-USDT",
			InstrumentType:      okexapi.InstrumentTypeSpot,
			OrderId:             strint.Int64(orderId),
			OrderType:           okexapi.OrderTypeLimit,
			Price:               fixedpoint.NewFromFloat(48174.5),
			Side:                okexapi.SideTypeBuy,
			State:               okexapi.OrderStateLive,
			Size:                fixedpoint.NewFromFloat(0.00001),
			UpdatedTime:         types.NewMillisecondTimestampFromInt(1704957916401),
		}
		expOrder = &types.Order{
			SubmitOrder: types.SubmitOrder{
				ClientOrderID: openOrder.ClientOrderId,
				Symbol:        toGlobalSymbol(openOrder.InstrumentID),
				Side:          types.SideTypeBuy,
				Type:          types.OrderTypeLimit,
				Quantity:      fixedpoint.NewFromFloat(0.00001),
				Price:         fixedpoint.NewFromFloat(48174.5),
				AveragePrice:  fixedpoint.Zero,
				StopPrice:     fixedpoint.Zero,
				TimeInForce:   types.TimeInForceGTC,
			},
			Exchange:         types.ExchangeOKEx,
			OrderID:          uint64(orderId),
			Status:           types.OrderStatusNew,
			OriginalStatus:   string(okexapi.OrderStateLive),
			ExecutedQuantity: fixedpoint.Zero,
			IsWorking:        true,
			CreationTime:     types.Time(types.NewMillisecondTimestampFromInt(1704957916401).Time()),
			UpdateTime:       types.Time(types.NewMillisecondTimestampFromInt(1704957916401).Time()),
		}
	)

	t.Run("succeeds", func(t *testing.T) {
		order, err := orderDetailToGlobalOrder(openOrder)
		assert.NoError(err)
		assert.Equal(expOrder, order)
	})

	t.Run("succeeds with market/buy/targetQuoteCurrency", func(t *testing.T) {
		newOrder := *openOrder
		newOrder.OrderType = okexapi.OrderTypeMarket
		newOrder.Side = okexapi.SideTypeBuy
		newOrder.TargetCurrency = okexapi.TargetCurrencyQuote
		newOrder.FillPrice = fixedpoint.NewFromFloat(100)
		newOrder.Size = fixedpoint.NewFromFloat(10000)
		newOrder.State = okexapi.OrderStatePartiallyFilled

		newExpOrder := *expOrder
		newExpOrder.Side = types.SideTypeBuy
		newExpOrder.Type = types.OrderTypeMarket
		newExpOrder.Quantity = fixedpoint.NewFromFloat(100)
		newExpOrder.Status = types.OrderStatusPartiallyFilled
		newExpOrder.OriginalStatus = string(okexapi.OrderStatePartiallyFilled)
		order, err := orderDetailToGlobalOrder(&newOrder)
		assert.NoError(err)
		assert.Equal(&newExpOrder, order)
	})

	t.Run("unexpected order status", func(t *testing.T) {
		newOrder := *openOrder
		newOrder.State = "xxx"
		_, err := orderDetailToGlobalOrder(&newOrder)
		assert.ErrorContains(err, "xxx")
	})

	t.Run("unexpected order type", func(t *testing.T) {
		newOrder := *openOrder
		newOrder.OrderType = "xxx"
		_, err := orderDetailToGlobalOrder(&newOrder)
		assert.ErrorContains(err, "xxx")
	})

}

func Test_tradeToGlobal(t *testing.T) {
	var (
		assert = assert.New(t)
		raw    = `{"side":"sell","fillSz":"1","fillPx":"46446.4","fillPxVol":"","fillFwdPx":"","fee":"-46","fillPnl":"0","ordId":"665951654130348158","feeRate":"-0.001","instType":"SPOT","fillPxUsd":"","instId":"BTC-USDT","clOrdId":"","posSide":"net","billId":"665951654138736652","fillMarkVol":"","tag":"","fillTime":"1705047247128","execType":"T","fillIdxPx":"","tradeId":"724072849","fillMarkPx":"","feeCcy":"USDT","ts":"1705047247130"}`
	)
	var res okexapi.Trade
	err := json.Unmarshal([]byte(raw), &res)
	assert.NoError(err)

	t.Run("succeeds with sell/taker", func(t *testing.T) {
		assert.Equal(toGlobalTrade(res), types.Trade{
			ID:            uint64(724072849),
			OrderID:       uint64(665951654130348158),
			Exchange:      types.ExchangeOKEx,
			Price:         fixedpoint.NewFromFloat(46446.4),
			Quantity:      fixedpoint.One,
			QuoteQuantity: fixedpoint.NewFromFloat(46446.4),
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeSell,
			IsBuyer:       false,
			IsMaker:       false,
			Time:          types.Time(types.NewMillisecondTimestampFromInt(1705047247130).Time()),
			Fee:           fixedpoint.NewFromFloat(46),
			FeeCurrency:   "USDT",
		})
	})

	t.Run("succeeds with buy/taker", func(t *testing.T) {
		newRes := res
		newRes.Side = okexapi.SideTypeBuy
		assert.Equal(toGlobalTrade(newRes), types.Trade{
			ID:            uint64(724072849),
			OrderID:       uint64(665951654130348158),
			Exchange:      types.ExchangeOKEx,
			Price:         fixedpoint.NewFromFloat(46446.4),
			Quantity:      fixedpoint.One,
			QuoteQuantity: fixedpoint.NewFromFloat(46446.4),
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			IsBuyer:       true,
			IsMaker:       false,
			Time:          types.Time(types.NewMillisecondTimestampFromInt(1705047247130).Time()),
			Fee:           fixedpoint.NewFromFloat(46),
			FeeCurrency:   "USDT",
		})
	})

	t.Run("succeeds with sell/maker", func(t *testing.T) {
		newRes := res
		newRes.ExecutionType = okexapi.LiquidityTypeMaker
		assert.Equal(toGlobalTrade(newRes), types.Trade{
			ID:            uint64(724072849),
			OrderID:       uint64(665951654130348158),
			Exchange:      types.ExchangeOKEx,
			Price:         fixedpoint.NewFromFloat(46446.4),
			Quantity:      fixedpoint.One,
			QuoteQuantity: fixedpoint.NewFromFloat(46446.4),
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeSell,
			IsBuyer:       false,
			IsMaker:       true,
			Time:          types.Time(types.NewMillisecondTimestampFromInt(1705047247130).Time()),
			Fee:           fixedpoint.NewFromFloat(46),
			FeeCurrency:   "USDT",
		})
	})

	t.Run("succeeds with buy/maker", func(t *testing.T) {
		newRes := res
		newRes.Side = okexapi.SideTypeBuy
		newRes.ExecutionType = okexapi.LiquidityTypeMaker
		assert.Equal(toGlobalTrade(newRes), types.Trade{
			ID:            uint64(724072849),
			OrderID:       uint64(665951654130348158),
			Exchange:      types.ExchangeOKEx,
			Price:         fixedpoint.NewFromFloat(46446.4),
			Quantity:      fixedpoint.One,
			QuoteQuantity: fixedpoint.NewFromFloat(46446.4),
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			IsBuyer:       true,
			IsMaker:       true,
			Time:          types.Time(types.NewMillisecondTimestampFromInt(1705047247130).Time()),
			Fee:           fixedpoint.NewFromFloat(46),
			FeeCurrency:   "USDT",
		})
	})
}

func Test_processMarketBuyQuantity(t *testing.T) {
	var (
		assert = assert.New(t)
	)

	t.Run("zero", func(t *testing.T) {
		size, err := processMarketBuySize(&okexapi.OrderDetail{State: okexapi.OrderStateLive})
		assert.NoError(err)
		assert.Equal(fixedpoint.Zero, size)

		size, err = processMarketBuySize(&okexapi.OrderDetail{State: okexapi.OrderStateCanceled})
		assert.NoError(err)
		assert.Equal(fixedpoint.Zero, size)
	})

	t.Run("estimated size", func(t *testing.T) {
		size, err := processMarketBuySize(&okexapi.OrderDetail{
			FillPrice: fixedpoint.NewFromFloat(2),
			Size:      fixedpoint.NewFromFloat(4),
			State:     okexapi.OrderStatePartiallyFilled,
		})
		assert.NoError(err)
		assert.Equal(fixedpoint.NewFromFloat(2), size)
	})

	t.Run("unexpected fill price", func(t *testing.T) {
		_, err := processMarketBuySize(&okexapi.OrderDetail{
			FillPrice: fixedpoint.Zero,
			Size:      fixedpoint.NewFromFloat(4),
			State:     okexapi.OrderStatePartiallyFilled,
		})
		assert.ErrorContains(err, "fillPrice")
	})

	t.Run("accumulatedFillsize", func(t *testing.T) {
		size, err := processMarketBuySize(&okexapi.OrderDetail{
			AccumulatedFillSize: fixedpoint.NewFromFloat(1000),
			State:               okexapi.OrderStateFilled,
		})
		assert.NoError(err)
		assert.Equal(fixedpoint.NewFromFloat(1000), size)
	})

	t.Run("unexpected status", func(t *testing.T) {
		_, err := processMarketBuySize(&okexapi.OrderDetail{
			State: "XXXXXXX",
		})
		assert.ErrorContains(err, "unexpected")
	})
}

func Test_toGlobalSymbol(t *testing.T) {
	symbol := "BTC-USDT"
	assert.Equal(t, "BTCUSDT", toGlobalSymbol(symbol))

	symbol = "BTC-USDT-SWAP"
	assert.Equal(t, "BTCUSDT", toGlobalSymbol(symbol))
}

func Test_toLocalSymbol(t *testing.T) {
	symbol := "BTCUSDT"
	assert.Equal(t, "BTC-USDT", toLocalSymbol(symbol))
	assert.Equal(t, "BTC-USDT-SWAP", toLocalSymbol(symbol, okexapi.InstrumentTypeSwap))
}

func Test_toGlobalOrder(t *testing.T) {
	localOrder := &okexapi.OrderDetails{
		OrderID:        "665576973905014786",
		Currency:       "BTC",
		Fee:            fixedpoint.Zero,
		FeeCurrency:    "USDT",
		InstrumentID:   "BTC-USDT",
		InstrumentType: okexapi.InstrumentTypeSpot,
		OrderType:      okexapi.OrderTypeLimit,
		Price:          fixedpoint.NewFromFloat(48174.5),
		Side:           okexapi.SideTypeBuy,
		State:          okexapi.OrderStateLive,
	}

	globalOrder, err := toGlobalOrder(localOrder)
	assert.NoError(t, err)
	assert.False(t, globalOrder.IsFutures)

	localOrder.InstrumentType = okexapi.InstrumentTypeSwap
	globalOrder, err = toGlobalOrder(localOrder)
	assert.NoError(t, err)
	assert.True(t, globalOrder.IsFutures)
}

func Test_toGlobalCurrency(t *testing.T) {
	tests := []struct {
		name      string
		record    okexapi.InstrumentInfo
		wantBase  string
		wantQuote string
	}{
		{
			name: "get from BaseCurrency and QuoteCurrency",
			record: okexapi.InstrumentInfo{
				BaseCurrency:  "BTC",
				QuoteCurrency: "USDT",
				InstrumentID:  "BTC-USDT",
			},
			wantBase:  "BTC",
			wantQuote: "USDT",
		},
		{
			name: "parse from InstrumentID",
			record: okexapi.InstrumentInfo{
				BaseCurrency:  "",
				QuoteCurrency: "",
				InstrumentID:  "BTC-USDT",
			},
			wantBase:  "BTC",
			wantQuote: "USDT",
		},
		{
			name: "parse from InstrumentID with SWAP suffix",
			record: okexapi.InstrumentInfo{
				BaseCurrency:  "",
				QuoteCurrency: "",
				InstrumentID:  "BTC-USDT-SWAP",
			},
			wantBase:  "BTC",
			wantQuote: "USDT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBase, gotQuote := toGlobalCurrency(tt.record)
			if gotBase != tt.wantBase {
				t.Errorf("toGlobalCurrency() gotBase = %v, want %v", gotBase, tt.wantBase)
			}
			if gotQuote != tt.wantQuote {
				t.Errorf("toGlobalCurrency() gotQuote = %v, want %v", gotQuote, tt.wantQuote)
			}
		})
	}
}

func Test_convertSubscription(t *testing.T) {
	tests := []struct {
		name     string
		sub      types.Subscription
		instType okexapi.InstrumentType
		want     WebsocketSubscription
		wantErr  bool
	}{
		{
			name: "convert kline subscription",
			sub: types.Subscription{
				Channel: types.KLineChannel,
				Symbol:  "BTCUSDT",
				Options: types.SubscribeOptions{
					Interval: types.Interval1h,
				},
			},
			instType: okexapi.InstrumentTypeSpot,
			want: WebsocketSubscription{
				Channel:      "candle1H",
				InstrumentID: "BTC-USDT",
			},
			wantErr: false,
		},
		{
			name: "convert book subscription with full depth",
			sub: types.Subscription{
				Channel: types.BookChannel,
				Symbol:  "BTCUSDT",
				Options: types.SubscribeOptions{
					Depth: types.DepthLevelFull,
				},
			},
			instType: okexapi.InstrumentTypeSpot,
			want: WebsocketSubscription{
				Channel:      ChannelBooks,
				InstrumentID: "BTC-USDT",
			},
			wantErr: false,
		},
		{
			name: "convert book subscription with medium depth",
			sub: types.Subscription{
				Channel: types.BookChannel,
				Symbol:  "BTCUSDT",
				Options: types.SubscribeOptions{
					Depth: types.DepthLevelMedium,
				},
			},
			instType: okexapi.InstrumentTypeSpot,
			want: WebsocketSubscription{
				Channel:      ChannelBooks50,
				InstrumentID: "BTC-USDT",
			},
			wantErr: false,
		},
		{
			name: "convert book ticker subscription",
			sub: types.Subscription{
				Channel: types.BookTickerChannel,
				Symbol:  "BTCUSDT",
			},
			instType: okexapi.InstrumentTypeSpot,
			want: WebsocketSubscription{
				Channel:      ChannelBooks5,
				InstrumentID: "BTC-USDT",
			},
			wantErr: false,
		},
		{
			name: "convert market trade subscription",
			sub: types.Subscription{
				Channel: types.MarketTradeChannel,
				Symbol:  "BTCUSDT",
			},
			instType: okexapi.InstrumentTypeSpot,
			want: WebsocketSubscription{
				Channel:      ChannelMarketTrades,
				InstrumentID: "BTC-USDT",
			},
			wantErr: false,
		},
		{
			name: "convert swap instrument subscription",
			sub: types.Subscription{
				Channel: types.KLineChannel,
				Symbol:  "BTCUSDT",
				Options: types.SubscribeOptions{
					Interval: types.Interval1h,
				},
			},
			instType: okexapi.InstrumentTypeSwap,
			want: WebsocketSubscription{
				Channel:      "candle1H",
				InstrumentID: "BTC-USDT-SWAP",
			},
			wantErr: false,
		},
		{
			name: "unsupported channel",
			sub: types.Subscription{
				Channel: "unsupported",
				Symbol:  "BTCUSDT",
			},
			instType: okexapi.InstrumentTypeSpot,
			want:     WebsocketSubscription{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertSubscription(tt.sub, tt.instType)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertSubscription() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertSubscription() = %v, want %v", got, tt.want)
			}
		})
	}
}
