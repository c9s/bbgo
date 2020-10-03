package types

import (
	"fmt"
	"sort"

	"github.com/c9s/bbgo/pkg/bbgo/fixedpoint"
)

type PriceVolume struct {
	Price  fixedpoint.Value
	Volume fixedpoint.Value
}

func (p PriceVolume) String() string {
	return fmt.Sprintf("PriceVolume{ price: %f, volume: %f }", p.Price.Float64(), p.Volume.Float64())
}

type PriceVolumeSlice []PriceVolume

func (slice PriceVolumeSlice) Len() int           { return len(slice) }
func (slice PriceVolumeSlice) Less(i, j int) bool { return slice[i].Price < slice[j].Price }
func (slice PriceVolumeSlice) Swap(i, j int)      { slice[i], slice[j] = slice[j], slice[i] }

// Trim removes the pairs that volume = 0
func (slice PriceVolumeSlice) Trim() (pvs PriceVolumeSlice) {
	for _, pv := range slice {
		if pv.Volume > 0 {
			pvs = append(pvs, pv)
		}
	}

	return pvs
}

func (slice PriceVolumeSlice) Copy() PriceVolumeSlice {
	// this is faster than make
	return append(slice[:0:0], slice...)
}

func (slice PriceVolumeSlice) InsertAt(idx int, pv PriceVolume) PriceVolumeSlice {
	rear := append([]PriceVolume{}, slice[idx:]...)
	slice = append(slice[:idx], pv)
	return append(slice, rear...)
}

func (slice PriceVolumeSlice) Remove(price fixedpoint.Value, descending bool) PriceVolumeSlice {
	matched, idx := slice.Find(price, descending)
	if matched.Price != price {
		return slice
	}

	return append(slice[:idx], slice[idx+1:]...)
}

// FindPriceVolumePair finds the pair by the given price, this function is a read-only
// operation, so we use the value receiver to avoid copy value from the pointer
// If the price is not found, it will return the index where the price can be inserted at.
// true for descending (bid orders), false for ascending (ask orders)
func (slice PriceVolumeSlice) Find(price fixedpoint.Value, descending bool) (pv PriceVolume, idx int) {
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

func (slice PriceVolumeSlice) Upsert(pv PriceVolume, descending bool) PriceVolumeSlice {
	if len(slice) == 0 {
		return append(slice, pv)
	}

	price := pv.Price
	_, idx := slice.Find(price, descending)
	if idx >= len(slice) || slice[idx].Price != price {
		return slice.InsertAt(idx, pv)
	}

	slice[idx].Volume = pv.Volume
	return slice
}

type OrderBook struct {
	Symbol string
	Bids   PriceVolumeSlice
	Asks   PriceVolumeSlice
}

func (b *OrderBook) UpdateAsks(pvs PriceVolumeSlice) {
	for _, pv := range pvs {
		if pv.Volume == 0 {
			b.Asks = b.Asks.Remove(pv.Price, false)
		} else {
			b.Asks = b.Asks.Upsert(pv, false)
		}
	}
}

func (b *OrderBook) UpdateBids(pvs PriceVolumeSlice) {
	for _, pv := range pvs {
		if pv.Volume == 0 {
			b.Bids = b.Bids.Remove(pv.Price, true)
		} else {
			b.Bids = b.Bids.Upsert(pv, true)
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
	for i := len(b.Asks) - 1; i >= 0; i-- {
		fmt.Printf("- ASK: %s\n", b.Asks[i].String())
	}

	fmt.Printf("BIDS:\n")
	for _, bid := range b.Bids {
		fmt.Printf("- BID: %s\n", bid.String())
	}
}
