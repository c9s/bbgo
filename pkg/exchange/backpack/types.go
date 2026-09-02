package backpack

import (
	"github.com/c9s/bbgo/pkg/exchange/backpack/backpackapi"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	// symbolSeparator separates the base and the quote currency in a Backpack symbol.
	symbolSeparator = "_"

	// perpSuffix marks a perpetual futures market, e.g. SOL_USDC_PERP.
	perpSuffix = "_PERP"
)

var (
	// SupportedIntervals are the kline intervals Backpack offers, in seconds.
	//
	// Backpack additionally supports 8h, which bbgo has no Interval constant for, and does not
	// support bbgo's 2w.
	SupportedIntervals = map[types.Interval]int{
		types.Interval1s:  1,
		types.Interval1m:  1 * 60,
		types.Interval3m:  3 * 60,
		types.Interval5m:  5 * 60,
		types.Interval15m: 15 * 60,
		types.Interval30m: 30 * 60,
		types.Interval1h:  60 * 60,
		types.Interval2h:  60 * 60 * 2,
		types.Interval4h:  60 * 60 * 4,
		types.Interval6h:  60 * 60 * 6,
		types.Interval12h: 60 * 60 * 12,
		types.Interval1d:  60 * 60 * 24,
		types.Interval3d:  60 * 60 * 24 * 3,
		types.Interval1w:  60 * 60 * 24 * 7,
		types.Interval1mo: 60 * 60 * 24 * 30,
	}

	// localIntervals maps a bbgo interval to the Backpack interval string.
	// Every interval keeps its name except 1mo, which Backpack spells "1month".
	localIntervals = map[types.Interval]backpackapi.KlineInterval{
		types.Interval1s:  backpackapi.KlineInterval1s,
		types.Interval1m:  backpackapi.KlineInterval1m,
		types.Interval3m:  backpackapi.KlineInterval3m,
		types.Interval5m:  backpackapi.KlineInterval5m,
		types.Interval15m: backpackapi.KlineInterval15m,
		types.Interval30m: backpackapi.KlineInterval30m,
		types.Interval1h:  backpackapi.KlineInterval1h,
		types.Interval2h:  backpackapi.KlineInterval2h,
		types.Interval4h:  backpackapi.KlineInterval4h,
		types.Interval6h:  backpackapi.KlineInterval6h,
		types.Interval12h: backpackapi.KlineInterval12h,
		types.Interval1d:  backpackapi.KlineInterval1d,
		types.Interval3d:  backpackapi.KlineInterval3d,
		types.Interval1w:  backpackapi.KlineInterval1w,
		types.Interval1mo: backpackapi.KlineInterval1month,
	}
)
