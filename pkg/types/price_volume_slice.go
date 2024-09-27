package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type PriceVolume struct {
	Price, Volume fixedpoint.Value
}

func NewPriceVolume(p, v fixedpoint.Value) PriceVolume {
	return PriceVolume{
		Price:  p,
		Volume: v,
	}
}

func (p PriceVolume) InQuote() fixedpoint.Value {
	return p.Price.Mul(p.Volume)
}

func (p PriceVolume) Equals(b PriceVolume) bool {
	return p.Price.Eq(b.Price) && p.Volume.Eq(b.Volume)
}

func (p PriceVolume) String() string {
	return fmt.Sprintf("PriceVolume{ Price: %s, Volume: %s }", p.Price.String(), p.Volume.String())
}

type PriceVolumeSlice []PriceVolume

func (slice PriceVolumeSlice) Len() int           { return len(slice) }
func (slice PriceVolumeSlice) Less(i, j int) bool { return slice[i].Price.Compare(slice[j].Price) < 0 }
func (slice PriceVolumeSlice) Swap(i, j int)      { slice[i], slice[j] = slice[j], slice[i] }

// Trim removes the pairs that volume = 0
func (slice PriceVolumeSlice) Trim() (pvs PriceVolumeSlice) {
	for _, pv := range slice {
		if pv.Volume.Sign() > 0 {
			pvs = append(pvs, pv)
		}
	}

	return pvs
}

func (slice PriceVolumeSlice) CopyDepth(depth int) PriceVolumeSlice {
	if depth == 0 || depth > len(slice) {
		return slice.Copy()
	}

	var s = make(PriceVolumeSlice, depth)
	copy(s, slice[:depth])
	return s
}

func (slice PriceVolumeSlice) Copy() PriceVolumeSlice {
	var s = make(PriceVolumeSlice, len(slice))
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

// ElemOrLast returns the element on the index i, if i is out of range, it will return the last element
func (slice PriceVolumeSlice) ElemOrLast(i int) (PriceVolume, bool) {
	if len(slice) == 0 {
		return PriceVolume{}, false
	}

	if i > len(slice)-1 {
		return slice[len(slice)-1], true
	}

	return slice[i], true
}

func (slice PriceVolumeSlice) IndexByQuoteVolumeDepth(requiredQuoteVolume fixedpoint.Value) int {
	var totalQuoteVolume = fixedpoint.Zero
	for x, pv := range slice {
		// this should use float64 multiply
		quoteVolume := fixedpoint.Mul(pv.Volume, pv.Price)
		totalQuoteVolume = totalQuoteVolume.Add(quoteVolume)
		if totalQuoteVolume.Compare(requiredQuoteVolume) >= 0 {
			return x
		}
	}

	// depth not enough
	return -1
}

func (slice PriceVolumeSlice) SumDepth() fixedpoint.Value {
	var total = fixedpoint.Zero
	for _, pv := range slice {
		total = total.Add(pv.Volume)
	}

	return total
}

func (slice PriceVolumeSlice) SumDepthInQuote() fixedpoint.Value {
	var total = fixedpoint.Zero

	for _, pv := range slice {
		quoteVolume := fixedpoint.Mul(pv.Price, pv.Volume)
		total = total.Add(quoteVolume)
	}

	return total
}

func (slice PriceVolumeSlice) IndexByVolumeDepth(requiredVolume fixedpoint.Value) int {
	var tv = fixedpoint.Zero
	for x, el := range slice {
		tv = tv.Add(el.Volume)
		if tv.Compare(requiredVolume) >= 0 {
			return x
		}
	}

	// depth not enough
	return -1
}

func (slice PriceVolumeSlice) InsertAt(idx int, pv PriceVolume) PriceVolumeSlice {
	rear := append([]PriceVolume{}, slice[idx:]...)
	newSlice := append(slice[:idx], pv)
	return append(newSlice, rear...)
}

func (slice PriceVolumeSlice) Remove(price fixedpoint.Value, descending bool) PriceVolumeSlice {
	matched, idx := slice.Find(price, descending)
	if matched.Price.Compare(price) != 0 || matched.Price.IsZero() {
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
			return slice[i].Price.Compare(price) <= 0
		}
		return slice[i].Price.Compare(price) >= 0
	})

	if idx >= len(slice) || slice[idx].Price.Compare(price) != 0 {
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
	if idx >= len(slice) || slice[idx].Price.Compare(price) != 0 {
		return slice.InsertAt(idx, pv)
	}

	slice[idx].Volume = pv.Volume
	return slice
}

func (slice *PriceVolumeSlice) UnmarshalJSON(b []byte) error {
	s, err := ParsePriceVolumeSliceJSON(b)
	if err != nil {
		return err
	}

	*slice = s
	return nil
}

// ParsePriceVolumeKvSliceJSON parses a JSON array of objects into PriceVolumeSlice
// [{"Price":...,"Volume":...}, ...]
func ParsePriceVolumeKvSliceJSON(b []byte) (PriceVolumeSlice, error) {
	type S PriceVolumeSlice
	var ts S

	err := json.Unmarshal(b, &ts)
	if err != nil {
		return nil, err
	}

	if len(ts) > 0 && ts[0].Price.IsZero() {
		return nil, fmt.Errorf("unable to parse price volume slice correctly, input given: %s", string(b))
	}

	return PriceVolumeSlice(ts), nil
}

// ParsePriceVolumeSliceJSON tries to parse a 2 dimensional string array into a PriceVolumeSlice
//
//	[["9000", "10"], ["9900", "10"], ... ]
//
// if parse failed, then it will try to parse the JSON array of objects, function ParsePriceVolumeKvSliceJSON will be called.
func ParsePriceVolumeSliceJSON(b []byte) (slice PriceVolumeSlice, err error) {
	var as [][]fixedpoint.Value

	err = json.Unmarshal(b, &as)
	if err != nil {
		// fallback unmarshalling: if the prefix looks like an object array
		if bytes.HasPrefix(b, []byte(`[{`)) {
			return ParsePriceVolumeKvSliceJSON(b)
		}

		return nil, err
	}

	for _, a := range as {
		var pv PriceVolume
		pv.Price = a[0]
		pv.Volume = a[1]

		// kucoin returns price in 0, we should skip
		if pv.Price.Eq(fixedpoint.Zero) {
			continue
		}

		slice = append(slice, pv)
	}

	return slice, nil
}

func (slice PriceVolumeSlice) AverageDepthPriceByQuote(requiredDepthInQuote fixedpoint.Value, maxLevel int) fixedpoint.Value {
	if len(slice) == 0 {
		return fixedpoint.Zero
	}

	totalQuoteAmount := fixedpoint.Zero
	totalQuantity := fixedpoint.Zero

	l := len(slice)
	if maxLevel > 0 && l > maxLevel {
		l = maxLevel
	}

	for i := 0; i < l; i++ {
		pv := slice[i]
		quoteAmount := fixedpoint.Mul(pv.Volume, pv.Price)
		totalQuoteAmount = totalQuoteAmount.Add(quoteAmount)
		totalQuantity = totalQuantity.Add(pv.Volume)

		if requiredDepthInQuote.Sign() > 0 && totalQuoteAmount.Compare(requiredDepthInQuote) > 0 {
			return totalQuoteAmount.Div(totalQuantity)
		}
	}

	return totalQuoteAmount.Div(totalQuantity)
}

// AverageDepthPrice uses the required total quantity to calculate the corresponding price
func (slice PriceVolumeSlice) AverageDepthPrice(requiredQuantity fixedpoint.Value) fixedpoint.Value {
	// rest quantity
	rq := requiredQuantity
	totalAmount := fixedpoint.Zero

	if len(slice) == 0 {
		return fixedpoint.Zero
	} else if slice[0].Volume.Compare(requiredQuantity) >= 0 {
		return slice[0].Price
	}

	for i := 0; i < len(slice); i++ {
		pv := slice[i]
		if pv.Volume.Compare(rq) >= 0 {
			totalAmount = totalAmount.Add(rq.Mul(pv.Price))
			break
		}

		rq = rq.Sub(pv.Volume)
		totalAmount = totalAmount.Add(pv.Volume.Mul(pv.Price))
	}

	return totalAmount.Div(requiredQuantity.Sub(rq))
}
