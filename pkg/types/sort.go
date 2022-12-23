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

// SortOrdersAscending sorts by creation time ascending-ly
func SortOrdersAscending(orders []Order) []Order {
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].CreationTime.Time().Before(orders[j].CreationTime.Time())
	})
	return orders
}

// SortOrdersAscending sorts by update time ascending-ly
func SortOrdersUpdateTimeAscending(orders []Order) []Order {
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].UpdateTime.Time().Before(orders[j].UpdateTime.Time())
	})
	return orders
}

func SortKLinesAscending(klines []KLine) []KLine {
	sort.Slice(klines, func(i, j int) bool {
		return klines[i].StartTime.Unix() < klines[j].StartTime.Unix()
	})

	return klines
}
