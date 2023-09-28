package riskcontrol

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type PriceRiskControl struct {
	referencePrice *indicatorv2.EWMAStream
	lossThreshold  fixedpoint.Value
}

func NewPriceRiskControl(refPrice *indicatorv2.EWMAStream, threshold fixedpoint.Value) *PriceRiskControl {
	return &PriceRiskControl{
		referencePrice: refPrice,
		lossThreshold:  threshold,
	}
}

func (r *PriceRiskControl) IsSafe(side types.SideType, price fixedpoint.Value, quantity fixedpoint.Value) bool {
	refPrice := fixedpoint.NewFromFloat(r.referencePrice.Last(0))
	// calculate profit
	var profit fixedpoint.Value
	if side == types.SideTypeBuy {
		profit = refPrice.Sub(price).Mul(quantity)
	} else if side == types.SideTypeSell {
		profit = price.Sub(refPrice).Mul(quantity)
	}
	return profit.Compare(r.lossThreshold) > 0
}

func (r *PriceRiskControl) IsSafeLimitOrder(o types.SubmitOrder) (bool, error) {
	if !(o.Type == types.OrderTypeLimit || o.Type == types.OrderTypeLimitMaker) {
		return false, fmt.Errorf("order type is not limit order")
	}
	return r.IsSafe(o.Side, o.Price, o.Quantity), nil
}
