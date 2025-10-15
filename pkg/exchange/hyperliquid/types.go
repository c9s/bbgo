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
