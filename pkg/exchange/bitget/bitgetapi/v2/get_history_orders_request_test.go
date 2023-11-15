package bitgetapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestOrderDetail_UnmarshalJSON(t *testing.T) {
	var (
		assert = assert.New(t)
	)
	t.Run("empty fee", func(t *testing.T) {
		input := `{
      "userId":"8672173294",
      "symbol":"APEUSDT",
      "orderId":"1104342023170068480",
      "clientOid":"f3d6a1ee-4e94-48b5-a6e0-25f3e93d92e1",
      "price":"1.2000000000000000",
      "size":"5.0000000000000000",
      "orderType":"limit",
      "side":"buy",
      "status":"cancelled",
      "priceAvg":"0",
      "baseVolume":"0.0000000000000000",
      "quoteVolume":"0.0000000000000000",
      "enterPointSource":"API",
      "feeDetail":"",
      "orderSource":"normal",
      "cTime":"1699021576683",
      "uTime":"1699021649099"
   }`
		var od OrderDetail
		err := json.Unmarshal([]byte(input), &od)
		assert.NoError(err)
		assert.Equal(OrderDetail{
			UserId:           types.StrInt64(8672173294),
			Symbol:           "APEUSDT",
			OrderId:          types.StrInt64(1104342023170068480),
			ClientOrderId:    "f3d6a1ee-4e94-48b5-a6e0-25f3e93d92e1",
			Price:            fixedpoint.NewFromFloat(1.2),
			Size:             fixedpoint.NewFromFloat(5),
			OrderType:        OrderTypeLimit,
			Side:             SideTypeBuy,
			Status:           OrderStatusCancelled,
			PriceAvg:         fixedpoint.Zero,
			BaseVolume:       fixedpoint.Zero,
			QuoteVolume:      fixedpoint.Zero,
			EnterPointSource: "API",
			FeeDetailRaw:     "",
			OrderSource:      "normal",
			CreatedTime:      types.NewMillisecondTimestampFromInt(1699021576683),
			UpdatedTime:      types.NewMillisecondTimestampFromInt(1699021649099),
			FeeDetail:        FeeDetail{},
		}, od)
	})

	t.Run("fee", func(t *testing.T) {
		input := `{
      "userId":"8672173294",
      "symbol":"APEUSDT",
      "orderId":"1104337778433757184",
      "clientOid":"8afea7bd-d873-44fe-aff8-6a1fae3cc765",
      "price":"1.4000000000000000",
      "size":"5.0000000000000000",
      "orderType":"limit",
      "side":"sell",
      "status":"filled",
      "priceAvg":"1.4001000000000000",
      "baseVolume":"5.0000000000000000",
      "quoteVolume":"7.0005000000000000",
      "enterPointSource":"API",
      "feeDetail":"{\"newFees\":{\"c\":0,\"d\":0,\"deduction\":false,\"r\":-0.0070005,\"t\":-0.0070005,\"totalDeductionFee\":0},\"USDT\":{\"deduction\":false,\"feeCoinCode\":\"USDT\",\"totalDeductionFee\":0,\"totalFee\":-0.007000500000}}",
      "orderSource":"normal",
      "cTime":"1699020564659",
      "uTime":"1699020564688"
   }`
		var od OrderDetail
		err := json.Unmarshal([]byte(input), &od)
		assert.NoError(err)
		assert.Equal(OrderDetail{
			UserId:           types.StrInt64(8672173294),
			Symbol:           "APEUSDT",
			OrderId:          types.StrInt64(1104337778433757184),
			ClientOrderId:    "8afea7bd-d873-44fe-aff8-6a1fae3cc765",
			Price:            fixedpoint.NewFromFloat(1.4),
			Size:             fixedpoint.NewFromFloat(5),
			OrderType:        OrderTypeLimit,
			Side:             SideTypeSell,
			Status:           OrderStatusFilled,
			PriceAvg:         fixedpoint.NewFromFloat(1.4001),
			BaseVolume:       fixedpoint.NewFromFloat(5),
			QuoteVolume:      fixedpoint.NewFromFloat(7.0005),
			EnterPointSource: "API",
			FeeDetailRaw:     `{"newFees":{"c":0,"d":0,"deduction":false,"r":-0.0070005,"t":-0.0070005,"totalDeductionFee":0},"USDT":{"deduction":false,"feeCoinCode":"USDT","totalDeductionFee":0,"totalFee":-0.007000500000}}`,
			OrderSource:      "normal",
			CreatedTime:      types.NewMillisecondTimestampFromInt(1699020564659),
			UpdatedTime:      types.NewMillisecondTimestampFromInt(1699020564688),
			FeeDetail: FeeDetail{
				NewFees: struct {
					DeductedByCoupon     fixedpoint.Value `json:"c"`
					DeductedInBGB        fixedpoint.Value `json:"d"`
					DeductedFromCurrency fixedpoint.Value `json:"r"`
					ToBePaid             fixedpoint.Value `json:"t"`
					Deduction            bool             `json:"deduction"`
					TotalDeductionFee    fixedpoint.Value `json:"totalDeductionFee"`
				}{DeductedByCoupon: fixedpoint.NewFromFloat(0),
					DeductedInBGB:        fixedpoint.NewFromFloat(0),
					DeductedFromCurrency: fixedpoint.NewFromFloat(-0.0070005),
					ToBePaid:             fixedpoint.NewFromFloat(-0.0070005),
					Deduction:            false,
					TotalDeductionFee:    fixedpoint.Zero,
				},
			},
		}, od)
	})
}
