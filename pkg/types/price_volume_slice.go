package types

import (
	"sort"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

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

func (slice PriceVolumeSlice) CopyDepth(depth int) PriceVolumeSlice {
	if depth > len(slice) {
		return slice.Copy()
	}

	var s = make(PriceVolumeSlice, depth, depth)
	copy(s, slice[:depth])
	return s
}

func (slice PriceVolumeSlice) Copy() PriceVolumeSlice {
	var s = make(PriceVolumeSlice, len(slice), len(slice))
	copy(s, slice)
	return s
}

func (slice PriceVolumeSlice) Second() (PriceVolume, bool) {
	if len(slice) > 1 {
		return slice[1], true
	}
	return PriceVolume{}, false
}

func (slice PriceVolumeSlice) First() (PriceVolume, bool) {
	if len(slice) > 0 {
		return slice[0], true
	}
	return PriceVolume{}, false
}

func (slice PriceVolumeSlice) IndexByVolumeDepth(requiredVolume fixedpoint.Value) int {
	var tv fixedpoint.Value = 0
	for x, el := range slice {
		tv += el.Volume
		if tv >= requiredVolume {
			return x
		}
	}

	// not deep enough
	return -1
}

func (slice PriceVolumeSlice) InsertAt(idx int, pv PriceVolume) PriceVolumeSlice {
	rear := append([]PriceVolume{}, slice[idx:]...)
	newSlice := append(slice[:idx], pv)
	return append(newSlice, rear...)
}

func (slice PriceVolumeSlice) Remove(price fixedpoint.Value, descending bool) PriceVolumeSlice {
	matched, idx := slice.Find(price, descending)
	if matched.Price != price {
		return slice
	}

	return append(slice[:idx], slice[idx+1:]...)
}

// Find finds the pair by the given price, this function is a read-only
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
