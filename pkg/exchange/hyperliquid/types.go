package hyperliquid

import (
	"sync"

	"github.com/c9s/bbgo/pkg/types"
)

var (
	SupportedIntervals = map[types.Interval]int{
		types.Interval1m:  1 * 60,
		types.Interval3m:  3 * 60,
		types.Interval5m:  5 * 60,
		types.Interval15m: 15 * 60,
		types.Interval30m: 30 * 60,
		types.Interval1h:  60 * 60,
		types.Interval2h:  60 * 60 * 2,
		types.Interval4h:  60 * 60 * 4,
		types.Interval8h:  60 * 60 * 8,
		types.Interval12h: 60 * 60 * 12,
		types.Interval1d:  60 * 60 * 24,
		types.Interval3d:  60 * 60 * 24 * 3,
		types.Interval1w:  60 * 60 * 24 * 7,
		types.Interval1mo: 60 * 60 * 24 * 30,
	}

	ToLocalInterval = map[types.Interval]string{
		types.Interval1m:  "1m",
		types.Interval3m:  "3m",
		types.Interval5m:  "5m",
		types.Interval15m: "15m",
		types.Interval30m: "30m",
		types.Interval1h:  "1h",
		types.Interval2h:  "2h",
		types.Interval4h:  "4h",
		types.Interval8h:  "8h",
		types.Interval12h: "12h",
		types.Interval1d:  "1d",
		types.Interval3d:  "3d",
		types.Interval1w:  "1w",
		types.Interval1mo: "1M",
	}
)

var spotSymbolSyncMap sync.Map
var futuresSymbolSyncMap sync.Map

func init() {
	for key, val := range spotSymbolMap {
		spotSymbolSyncMap.Store(key, val)
	}

	for key, val := range futuresSymbolMap {
		futuresSymbolSyncMap.Store(key, val)
	}
}
