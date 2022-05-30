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

func SortOrdersAscending(orders []Order) []Order {
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].CreationTime.Time().Before(orders[j].CreationTime.Time())
	})
	return orders
}

func SortKLinesAscending(klines []KLine) []KLine {
	sort.Slice(klines, func(i, j int) bool {
		return klines[i].StartTime.Unix() < klines[j].StartTime.Unix()
	})

	return klines
}
