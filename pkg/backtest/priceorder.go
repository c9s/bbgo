package backtest

import (
	"sort"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type PriceOrder struct {
	Price fixedpoint.Value
	Order types.Order
}

type PriceOrderSlice []PriceOrder

func (slice PriceOrderSlice) Len() int           { return len(slice) }
func (slice PriceOrderSlice) Less(i, j int) bool { return slice[i].Price < slice[j].Price }
func (slice PriceOrderSlice) Swap(i, j int)      { slice[i], slice[j] = slice[j], slice[i] }

func (slice PriceOrderSlice) InsertAt(idx int, po PriceOrder) PriceOrderSlice {
	rear := append([]PriceOrder{}, slice[idx:]...)
	newSlice := append(slice[:idx], po)
	return append(newSlice, rear...)
}

func (slice PriceOrderSlice) Remove(price fixedpoint.Value, descending bool) PriceOrderSlice {
	matched, idx := slice.Find(price, descending)
	if matched.Price != price {
		return slice
	}

	return append(slice[:idx], slice[idx+1:]...)
}

func (slice PriceOrderSlice) First() (PriceOrder, bool) {
	if len(slice) > 0 {
		return slice[0], true
	}
	return PriceOrder{}, false
}

// FindPriceVolumePair finds the pair by the given price, this function is a read-only
// operation, so we use the value receiver to avoid copy value from the pointer
// If the price is not found, it will return the index where the price can be inserted at.
// true for descending (bid orders), false for ascending (ask orders)
func (slice PriceOrderSlice) Find(price fixedpoint.Value, descending bool) (pv PriceOrder, idx int) {
	idx = sort.Search(len(slice), func(i int) bool {
		if descending {
			return slice[i].Price <= price
		}
		return slice[i].Price >= price
	})

	if idx >= len(slice) || slice[idx].Price != price {
		return pv, idx
	}

	pv = slice[idx]

	return pv, idx
}

func (slice PriceOrderSlice) Upsert(po PriceOrder, descending bool) PriceOrderSlice {
	if len(slice) == 0 {
		return append(slice, po)
	}

	price := po.Price
	_, idx := slice.Find(price, descending)
	if idx >= len(slice) || slice[idx].Price != price {
		return slice.InsertAt(idx, po)
	}

	slice[idx].Order = po.Order
	return slice
}
