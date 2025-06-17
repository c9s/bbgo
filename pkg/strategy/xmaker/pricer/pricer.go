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
}

func NewCoveredDepth(depthBook *types.DepthBook, initialDepth fixedpoint.Value) *CoveredDepth {
	return &CoveredDepth{
		lastIndex:        -1,
		initialDepth:     initialDepth,
		depthBook:        depthBook,
		accumulatedDepth: fixedpoint.Zero,
	}
}

func (d *CoveredDepth) Cover(depth fixedpoint.Value) {
	if d.accumulatedDepth.IsZero() {
		d.accumulatedDepth = depth
	} else {
		d.accumulatedDepth = d.accumulatedDepth.Add(depth)
	}
}

func (d *CoveredDepth) Pricer(
	side types.SideType,
) Pricer {
	return func(i int, price fixedpoint.Value) fixedpoint.Value {
		if i <= d.lastIndex {
			// If the index is not increasing, we do not accumulate depth.
			// This is to prevent re-accumulating depth when the same index is processed again.
			log.Warnf("FromAccumulatedDepth: index %d is not increasing from last index %d, skipping accumulation", i, d.lastIndex)
			return d.depthBook.PriceAtDepth(side, d.initialDepth)
		}

		if d.lastIndex == 0 {
			// If this is the first index, we set the initial depth.
			price = d.depthBook.PriceAtDepth(side, d.initialDepth)
			d.accumulatedDepth = d.initialDepth
		} else {
			price = d.depthBook.PriceAtDepth(side, d.accumulatedDepth)
		}

		d.lastIndex = i
		return price
	}
}

func FromDepthBook(side types.SideType, depthBook *types.DepthBook, depth fixedpoint.Value) Pricer {
	return func(i int, price fixedpoint.Value) fixedpoint.Value {
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
