package xmaker

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Pricer func(i int, price fixedpoint.Value) fixedpoint.Value

func FromDepth(side types.SideType, depthBook *types.DepthBook, depth fixedpoint.Value) Pricer {
	return func(i int, price fixedpoint.Value) fixedpoint.Value {
		return depthBook.PriceAtDepth(side, depth)
	}
}

func ApplyMargin(side types.SideType, margin fixedpoint.Value) Pricer {
	if side == types.SideTypeSell {
		margin = margin.Neg()
	}

	return func(_ int, price fixedpoint.Value) fixedpoint.Value {
		return price.Mul(fixedpoint.One.Add(margin))
	}
}

func ApplyFeeRate(side types.SideType, feeRate fixedpoint.Value) Pricer {
	if side == types.SideTypeBuy {
		feeRate = feeRate.Neg()
	}

	return func(i int, price fixedpoint.Value) fixedpoint.Value {
		return price.Mul(fixedpoint.One.Add(feeRate))
	}
}

func AdjustByTick(side types.SideType, pips, tickSize fixedpoint.Value) Pricer {
	if side == types.SideTypeSell {
		tickSize = tickSize.Neg()
	}

	return func(i int, price fixedpoint.Value) fixedpoint.Value {
		if i == 0 {
			return price
		}

		if tickSize.IsZero() {
			return price
		}

		if i > 0 {
			price = price.Add(pips.Mul(tickSize))
		}

		return price
	}
}

func ComposePricers(pricers ...Pricer) Pricer {
	return func(i int, price fixedpoint.Value) fixedpoint.Value {
		for _, pricer := range pricers {
			price = pricer(i, price)
		}
		return price
	}
}
