package xfundingv2

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// directionPolicy encapsulates the per-direction decisions for a funding-rate
// arbitrage round so the rest of the round code can stay direction-agnostic.
//
// For PositionShort: long spot, short futures, collateral = base.
// For PositionLong:  short spot, long futures, collateral = quote.
type directionPolicy struct {
	Direction types.PositionType `json:"direction"`
	Market    types.Market       `json:"market"`
}

func newDirectionPolicy(direction types.PositionType, market types.Market) (directionPolicy, error) {
	switch direction {
	case types.PositionShort, types.PositionLong:
		return directionPolicy{Direction: direction, Market: market}, nil
	default:
		return directionPolicy{}, fmt.Errorf("unsupported futures direction: %q", direction)
	}
}

// CollateralAsset is the asset that gets parked on the futures account to
// support the futures leg of the round.
func (p directionPolicy) CollateralAsset() string {
	if p.Direction == types.PositionLong {
		return p.Market.QuoteCurrency
	}
	return p.Market.BaseCurrency
}

// TransferAmountFromSpotTrade returns how much CollateralAsset should be
// transferred from spot to futures after a spot fill during opening.
//
//	Short open: spot BUYs base -> transfer trade.Quantity of base.
//	Long  open: spot SELLs base for quote -> transfer net quote received
//	            (trade.QuoteQuantity minus fee when fee is paid in quote).
func (p directionPolicy) TransferAmountFromSpotTrade(t types.Trade) fixedpoint.Value {
	if p.Direction == types.PositionLong {
		amount := t.QuoteQuantity
		if amount.IsZero() {
			amount = t.Price.Mul(t.Quantity)
		}
		if t.FeeCurrency == p.Market.QuoteCurrency {
			amount = amount.Sub(t.Fee)
		}
		if amount.Sign() < 0 {
			return fixedpoint.Zero
		}
		return amount
	}
	return t.Quantity
}

// TransferAmountFromFuturesTrade returns how much CollateralAsset should be
// transferred from futures back to spot after a futures fill during closing.
//
//	Short close: futures BUYs to reduce short -> free trade.Quantity of base.
//	Long  close: futures SELLs to reduce long -> free trade.QuoteQuantity of quote.
func (p directionPolicy) TransferAmountFromFuturesTrade(t types.Trade) fixedpoint.Value {
	if p.Direction == types.PositionLong {
		return t.QuoteQuantity
	}
	return t.Quantity
}

// SpotSign / FuturesSign give the sign multipliers a caller should apply to a
// raw position size when constructing signed target positions for each leg.
//
//	Short: spot=+1 (long spot),  futures=-1 (short futures)
//	Long:  spot=-1 (short spot), futures=+1 (long futures)
func (p directionPolicy) SpotSign() int {
	if p.Direction == types.PositionLong {
		return -1
	}
	return 1
}

func (p directionPolicy) FuturesSign() int {
	return -p.SpotSign()
}
