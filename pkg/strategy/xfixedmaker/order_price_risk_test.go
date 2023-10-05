package xfixedmaker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_OrderPriceRiskControl_IsSafe(t *testing.T) {
	refPrice := 30000.00
	lossThreshold := fixedpoint.NewFromFloat(-100)

	window := types.IntervalWindow{Window: 30, Interval: types.Interval1m}
	refPriceEWMA := indicatorv2.EWMA2(nil, window.Window)
	refPriceEWMA.PushAndEmit(refPrice)

	cases := []struct {
		name     string
		side     types.SideType
		price    fixedpoint.Value
		quantity fixedpoint.Value
		isSafe   bool
	}{
		{
			name:     "BuyingHighSafe",
			side:     types.SideTypeBuy,
			price:    fixedpoint.NewFromFloat(30040.0),
			quantity: fixedpoint.NewFromFloat(1.0),
			isSafe:   true,
		},
		{
			name:     "SellingLowSafe",
			side:     types.SideTypeSell,
			price:    fixedpoint.NewFromFloat(29960.0),
			quantity: fixedpoint.NewFromFloat(1.0),
			isSafe:   true,
		},
		{
			name:     "BuyingHighLoss",
			side:     types.SideTypeBuy,
			price:    fixedpoint.NewFromFloat(30040.0),
			quantity: fixedpoint.NewFromFloat(10.0),
			isSafe:   false,
		},
		{
			name:     "SellingLowLoss",
			side:     types.SideTypeSell,
			price:    fixedpoint.NewFromFloat(29960.0),
			quantity: fixedpoint.NewFromFloat(10.0),
			isSafe:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var riskControl = NewOrderPriceRiskControl(refPriceEWMA, lossThreshold)
			assert.Equal(t, tc.isSafe, riskControl.IsSafe(tc.side, tc.price, tc.quantity))
		})
	}
}
