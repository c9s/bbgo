package okex

import "github.com/c9s/bbgo/pkg/types"

var (
	// below are supported UTC timezone interval for okex
	SupportedIntervals = map[types.Interval]int{
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

	ToGlobalInterval = map[string]types.Interval{
		"1m":  types.Interval1m,
		"3m":  types.Interval3m,
		"5m":  types.Interval5m,
		"15m": types.Interval15m,
		"30m": types.Interval30m,
		"1H":  types.Interval1h,
		"2H":  types.Interval2h,
		"4H":  types.Interval4h,
		"6H":  types.Interval6h,
		"12H": types.Interval12h,
		"1D":  types.Interval1d,
		"3D":  types.Interval3d,
		"1W":  types.Interval1w,
		"1M":  types.Interval1mo,
	}
)
