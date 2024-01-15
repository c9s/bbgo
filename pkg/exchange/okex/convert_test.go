package okex

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
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
			OrderId:             types.StrInt64(orderId),
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
			UUID:             fmt.Sprintf("%d", orderId),
			Status:           types.OrderStatusNew,
			OriginalStatus:   string(okexapi.OrderStateLive),
			ExecutedQuantity: fixedpoint.Zero,
			IsWorking:        true,
			CreationTime:     types.Time(types.NewMillisecondTimestampFromInt(1704957916401).Time()),
			UpdateTime:       types.Time(types.NewMillisecondTimestampFromInt(1704957916401).Time()),
		}
	)

	t.Run("succeeds", func(t *testing.T) {
		order, err := orderDetailToGlobal(openOrder)
		assert.NoError(err)
		assert.Equal(expOrder, order)
	})

	t.Run("unexpected order status", func(t *testing.T) {
		newOrder := *openOrder
		newOrder.State = "xxx"
		_, err := orderDetailToGlobal(&newOrder)
		assert.ErrorContains(err, "xxx")
	})

	t.Run("unexpected order type", func(t *testing.T) {
		newOrder := *openOrder
		newOrder.OrderType = "xxx"
		_, err := orderDetailToGlobal(&newOrder)
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
		assert.Equal(tradeToGlobal(res), types.Trade{
			ID:            uint64(665951654138736652),
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
		assert.Equal(tradeToGlobal(newRes), types.Trade{
			ID:            uint64(665951654138736652),
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
		assert.Equal(tradeToGlobal(newRes), types.Trade{
			ID:            uint64(665951654138736652),
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
		assert.Equal(tradeToGlobal(newRes), types.Trade{
			ID:            uint64(665951654138736652),
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
