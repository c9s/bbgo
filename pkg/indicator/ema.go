package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/store"
	"github.com/c9s/bbgo/pkg/types"
)

type EWMA struct {
	Interval string
	Window   int
	Values   Float64Slice
	EndTime  time.Time
}

func (inc *EWMA) calculateAndUpdate(kLines []types.KLine) {
	if len(kLines) < inc.Window {
		// we can't calculate
		return
	}

	var index = len(kLines) - 1
	var lastK = kLines[index]
	var multiplier = 2.0 / float64(inc.Window+1)

	if inc.EndTime != zeroTime && lastK.EndTime.Before(inc.EndTime) {
		return
	}

	var recentK = kLines[index-(inc.Window-1) : index+1]
	if len(inc.Values) > 0 {
		var previousEWMA = inc.Values[len(inc.Values)-1]
		var ewma = lastK.Close * multiplier + previousEWMA * (1 - multiplier)
		inc.Values.Push(ewma)
	} else {
		// The first EWMA is actually SMA
		var sma = calculateSMA(recentK)
		inc.Values.Push(sma)
	}

	inc.EndTime = kLines[index].EndTime
}

func (inc *EWMA) BindMarketDataStore(store *store.MarketDataStore) {
	store.OnKLineUpdate(func(kline types.KLine) {
		// kline guard
		if inc.Interval != kline.Interval {
			return
		}

		if inc.EndTime != zeroTime && inc.EndTime.Before(inc.EndTime) {
			return
		}

		if kLines, ok := store.KLinesOfInterval(types.Interval(kline.Interval)); ok {
			inc.calculateAndUpdate(kLines)
		}
	})
}
