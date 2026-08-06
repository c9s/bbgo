package pricer

import (
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Pricer func(i int, price fixedpoint.Value) fixedpoint.Value

type CoveredDepth struct {
	lastIndex        int
	accumulatedDepth fixedpoint.Value
	initialDepth     fixedpoint.Value
	depthBook        *types.DepthBook
	side             types.SideType

	// if useQuoteQuantity is true, the depth will be interpreted as quote quantity, not base quantity.
	useQuoteQuantity bool
}

// NewCoveredDepth creates a new CoveredDepth instance.
// If useQuoteQuantity is true, the instance will work under the quote depth mode, which means all the depth-related
// calculations will be based on quote quantity instead of base quantity.
// Note that it will interpret all the depth-related parameters that are passed to the instance as quote quantity as well.
func NewCoveredDepth(depthBook *types.DepthBook, side types.SideType, initialDepth fixedpoint.Value, useQuoteQuantity bool) *CoveredDepth {
	return &CoveredDepth{
		lastIndex:        -1,
		initialDepth:     initialDepth,
		depthBook:        depthBook,
		side:             side,
		accumulatedDepth: fixedpoint.Zero,
		useQuoteQuantity: useQuoteQuantity,
	}
}

func (d *CoveredDepth) Cover(depth fixedpoint.Value) {
	if d.accumulatedDepth.IsZero() {
		d.accumulatedDepth = depth
	} else {
		d.accumulatedDepth = d.accumulatedDepth.Add(depth)
	}
}

func (d *CoveredDepth) Pricer() Pricer {
	return func(i int, price fixedpoint.Value) fixedpoint.Value {
		depthFunc := d.depthBook.PriceAtDepth
		if d.useQuoteQuantity {
			depthFunc = d.depthBook.PriceAtQuoteDepth
		}
		if i <= d.lastIndex {
			// If the index is not increasing, we do not accumulate depth.
			// This is to prevent re-accumulating depth when the same index is processed again.
			log.Warnf("FromAccumulatedDepth: index %d is not increasing from last index %d, skipping accumulation", i, d.lastIndex)
			return depthFunc(d.side, d.initialDepth)
		}

		if d.lastIndex == 0 {
			// If this is the first index, we set the initial depth.
			price = depthFunc(d.side, d.initialDepth)
			d.accumulatedDepth = d.initialDepth
		} else {
			price = depthFunc(d.side, d.accumulatedDepth)
		}

		d.lastIndex = i
		return price
	}
}

func FromDepthBook(side types.SideType, depthBook *types.DepthBook, depth fixedpoint.Value, useQuoteQuantity bool) Pricer {
	return func(i int, price fixedpoint.Value) fixedpoint.Value {
		if useQuoteQuantity {
			return depthBook.PriceAtQuoteDepth(side, depth)
		}
		return depthBook.PriceAtDepth(side, depth)
	}
}

func FromBestPrice(side types.SideType, book *types.StreamOrderBook) Pricer {
	f := book.BestBid
	if side == types.SideTypeSell {
		f = book.BestAsk
	}

	return func(i int, price fixedpoint.Value) fixedpoint.Value {
		first, ok := f()
		if ok {
			return first.Price
		}

		return fixedpoint.Zero
	}
}

func ApplyMargin(side types.SideType, margin fixedpoint.Value) Pricer {
	if side == types.SideTypeBuy {
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

	if feeRate.IsZero() {
		return func(_ int, price fixedpoint.Value) fixedpoint.Value {
			return price
		}
	}

	return func(i int, price fixedpoint.Value) fixedpoint.Value {
		return price.Mul(fixedpoint.One.Add(feeRate))
	}
}

func AdjustByTick(side types.SideType, pips, tickSize fixedpoint.Value) Pricer {
	if side == types.SideTypeBuy {
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

func Compose(pricers ...Pricer) Pricer {
	if len(pricers) == 1 {
		return pricers[0]
	}

	return func(i int, price fixedpoint.Value) fixedpoint.Value {
		for _, pricer := range pricers {
			price = pricer(i, price)
		}
		return price
	}
}
