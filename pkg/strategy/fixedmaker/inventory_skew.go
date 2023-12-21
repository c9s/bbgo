package fixedmaker

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var (
	zero = fixedpoint.Zero
	two  = fixedpoint.NewFromFloat(2.0)
)

type InventorySkewBidAskRatios struct {
	BidRatio fixedpoint.Value
	AskRatio fixedpoint.Value
}

// https://hummingbot.org/strategy-configs/inventory-skew/
// https://github.com/hummingbot/hummingbot/blob/31fc61d5e71b2c15732142d30983f3ea2be4d466/hummingbot/strategy/pure_market_making/inventory_skew_calculator.pyx
type InventorySkew struct {
	InventoryRangeMultiplier fixedpoint.Value `json:"inventoryRangeMultiplier"`
	TargetBaseRatio          fixedpoint.Value `json:"targetBaseRatio"`
}

func (s *InventorySkew) Validate() error {
	if s.InventoryRangeMultiplier.Float64() < 0 {
		return fmt.Errorf("inventoryRangeMultiplier should be positive")
	}

	if s.TargetBaseRatio.Float64() < 0 {
		return fmt.Errorf("targetBaseRatio should be positive")
	}
	return nil
}

func (s *InventorySkew) CalculateBidAskRatios(quantity fixedpoint.Value, price fixedpoint.Value, baseBalance fixedpoint.Value, quoteBalance fixedpoint.Value) *InventorySkewBidAskRatios {
	baseValue := baseBalance.Mul(price)
	totalValue := baseValue.Add(quoteBalance)

	inventoryRange := s.InventoryRangeMultiplier.Mul(quantity.Mul(two)).Mul(price)
	leftLimit := s.TargetBaseRatio.Mul(totalValue).Sub(inventoryRange)
	rightLimit := s.TargetBaseRatio.Mul(totalValue).Add(inventoryRange)

	bidAdjustment := interp(baseValue, leftLimit, rightLimit, two, zero).Clamp(zero, two)
	askAdjustment := interp(baseValue, leftLimit, rightLimit, zero, two).Clamp(zero, two)

	return &InventorySkewBidAskRatios{
		BidRatio: bidAdjustment,
		AskRatio: askAdjustment,
	}
}

func interp(x, x0, x1, y0, y1 fixedpoint.Value) fixedpoint.Value {
	return y0.Add(x.Sub(x0).Mul(y1.Sub(y0)).Div(x1.Sub(x0)))
}
