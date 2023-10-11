package xfixedmaker

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type OrderPriceRiskControl struct {
	referencePrice *indicatorv2.EWMAStream
	lossThreshold  fixedpoint.Value
}

func NewOrderPriceRiskControl(referencePrice *indicatorv2.EWMAStream, threshold fixedpoint.Value) *OrderPriceRiskControl {
	return &OrderPriceRiskControl{
		referencePrice: referencePrice,
		lossThreshold:  threshold,
	}
}

func (r *OrderPriceRiskControl) IsSafe(side types.SideType, price fixedpoint.Value, quantity fixedpoint.Value) bool {
	refPrice := fixedpoint.NewFromFloat(r.referencePrice.Last(0))
	// calculate profit
	var profit fixedpoint.Value
	if side == types.SideTypeBuy {
		profit = refPrice.Sub(price).Mul(quantity)
	} else if side == types.SideTypeSell {
		profit = price.Sub(refPrice).Mul(quantity)
	} else {
		log.Warnf("OrderPriceRiskControl: unsupported side type: %s", side)
		return false
	}
	return profit.Compare(r.lossThreshold) > 0
}
