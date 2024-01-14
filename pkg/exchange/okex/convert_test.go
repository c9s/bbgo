package okex

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_openOrderToGlobal(t *testing.T) {
	var (
		assert = assert.New(t)

		orderId = 665576973905014786
		// {"accFillSz":"0","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"","cTime":"1704957916401","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"0","feeCcy":"USDT","fillPx":"","fillSz":"0","fillTime":"","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665576973905014786","ordType":"limit","pnl":"0","posSide":"net","px":"48174.5","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"BTC","reduceOnly":"false","side":"sell","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"live","stpId":"","stpMode":"","sz":"0.00001","tag":"","tdMode":"cash","tgtCcy":"","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"","uTime":"1704957916401"}
		openOrder = &okexapi.OpenOrder{
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
		order, err := openOrderToGlobal(openOrder)
		assert.NoError(err)
		assert.Equal(expOrder, order)
	})

	t.Run("unexpected order status", func(t *testing.T) {
		newOrder := *openOrder
		newOrder.State = "xxx"
		_, err := openOrderToGlobal(&newOrder)
		assert.ErrorContains(err, "xxx")
	})

	t.Run("unexpected order type", func(t *testing.T) {
		newOrder := *openOrder
		newOrder.OrderType = "xxx"
		_, err := openOrderToGlobal(&newOrder)
		assert.ErrorContains(err, "xxx")
	})

}
