package types

import (
	"fmt"
	"sort"

	"github.com/c9s/bbgo/pkg/bbgo/fixedpoint"
)

type PriceVolumePair struct {
	Price  fixedpoint.Value
	Volume fixedpoint.Value
}

func (p PriceVolumePair) String() string {
	return fmt.Sprintf("PriceVolumePair{ price: %f, volume: %f }", p.Price.Float64(), p.Volume.Float64())
}

type PriceVolumePairSlice []PriceVolumePair

func (ps PriceVolumePairSlice) Len() int           { return len(ps) }
func (ps PriceVolumePairSlice) Less(i, j int) bool { return ps[i].Price < ps[j].Price }
func (ps PriceVolumePairSlice) Swap(i, j int)      { ps[i], ps[j] = ps[j], ps[i] }

// Trim removes the pairs that volume = 0
func (ps PriceVolumePairSlice) Trim() (newps PriceVolumePairSlice) {
	for _, pv := range ps {
		if pv.Volume > 0 {
			newps = append(newps, pv)
		}
	}
	return newps
}

func (ps PriceVolumePairSlice) Copy() PriceVolumePairSlice {
	// this is faster than make
	return append(ps[:0:0], ps...)
}

func (ps *PriceVolumePairSlice) UpdateOrInsert(newPair PriceVolumePair, descending bool) {
	var newps = UpdateOrInsertPriceVolumePair(*ps, newPair, descending)
	*ps = newps
}

func (ps *PriceVolumePairSlice) RemoveByPrice(price fixedpoint.Value, descending bool) {
	var newps = RemovePriceVolumePair(*ps, price, descending)
	*ps = newps
}

// FindPriceVolumePair finds the pair by the given price, this function is a read-only
// operation, so we use the value receiver to avoid copy value from the pointer
// If the price is not found, it will return the index where the price can be inserted at.
// true for descending (bid orders), false for ascending (ask orders)
func FindPriceVolumePair(slice []PriceVolumePair, price fixedpoint.Value, descending bool) (int, PriceVolumePair) {
	idx := sort.Search(len(slice), func(i int) bool {
		if descending {
			return slice[i].Price <= price
		}
		return slice[i].Price >= price
	})
	if idx >= len(slice) || slice[idx].Price != price {
		return idx, PriceVolumePair{}
	}
	return idx, slice[idx]
}

//true for descending (bid orders), false for ascending (ask orders)
func UpdateOrInsertPriceVolumePair(slice []PriceVolumePair, pvPair PriceVolumePair, descending bool) []PriceVolumePair {
	price := pvPair.Price
	if len(slice) == 0 {
		return append(slice, pvPair)
	}

	idx, _ := FindPriceVolumePair(slice, price, descending)
	if idx >= len(slice) || slice[idx].Price != price {
		return InsertPriceVolumePairAt(slice, pvPair, idx)
	} else {
		slice[idx].Volume = pvPair.Volume
		return slice
	}
}

func InsertPriceVolumePairAt(slice []PriceVolumePair, pvPair PriceVolumePair, idx int) []PriceVolumePair {
	rear := append([]PriceVolumePair{}, slice[idx:]...)
	slice = append(slice[:idx], pvPair)
	return append(slice, rear...)
}

func RemovePriceVolumePair(slice []PriceVolumePair, price fixedpoint.Value, descending bool) []PriceVolumePair {
	idx, matched := FindPriceVolumePair(slice, price, descending)
	if matched.Price != price {
		return slice
	}

	return append(slice[:idx], slice[idx+1:]...)
}

type OrderBook struct {
	Symbol string
	Bids PriceVolumePairSlice
	Asks PriceVolumePairSlice
}

func (b *OrderBook) UpdateAsks(pvs PriceVolumePairSlice) {
	for _, pv := range pvs {
		if pv.Volume == 0 {
			b.Asks.RemoveByPrice(pv.Price, false)
		} else {
			b.Asks.UpdateOrInsert(pv, false)
		}
	}
}

func (b *OrderBook) UpdateBids(pvs PriceVolumePairSlice) {
	for _, pv := range pvs {
		if pv.Volume == 0 {
			b.Bids.RemoveByPrice(pv.Price, true)
		} else {
			b.Bids.UpdateOrInsert(pv, true)
		}
	}
}


func (b *OrderBook) Load(book OrderBook) {
	b.Bids = nil
	b.Asks = nil
	b.Update(book)
}

func (b *OrderBook) Update(book OrderBook) {
	b.UpdateBids(book.Bids)
	b.UpdateAsks(book.Asks)
}

func (b *OrderBook) Print() {
	fmt.Printf("BOOK %s\n", b.Symbol)
	fmt.Printf("ASKS:\n")
	for i := len(b.Asks) - 1 ; i >= 0 ; i-- {
		fmt.Printf("- ASK: %s\n", b.Asks[i].String())
	}

	fmt.Printf("BIDS:\n")
	for _, bid := range b.Bids {
		fmt.Printf("- BID: %s\n", bid.String())
	}
}

