package types

import (
	"sort"
	"time"
)

func SortTradesAscending(trades []Trade) []Trade {
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].Time.Before(time.Time(trades[j].Time))
	})
	return trades
}
