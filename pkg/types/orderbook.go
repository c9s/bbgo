package types

import (
	"fmt"
	"sort"
	"sync"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/sigchan"
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
	// this is faster than make (however it's only for simple types)
	return append(slice[:0:0], slice...)
}

func (slice PriceVolumeSlice) First() (PriceVolume, bool) {
	if len(slice) > 0 {
		return slice[0], true
	}
	return PriceVolume{}, false
}

func (slice PriceVolumeSlice) IndexByVolumeDepth(requiredVolume fixedpoint.Value) int {
	var tv int64 = 0
	for x, el := range slice {
		tv += el.Volume.Int64()
		if tv >= requiredVolume.Int64() {
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

//go:generate callbackgen -type OrderBook
type OrderBook struct {
	Symbol string
	Bids   PriceVolumeSlice
	Asks   PriceVolumeSlice

	loadCallbacks       []func(book *OrderBook)
	updateCallbacks     []func(book *OrderBook)
	bidsChangeCallbacks []func(pvs PriceVolumeSlice)
	asksChangeCallbacks []func(pvs PriceVolumeSlice)
}

func (b *OrderBook) BestBid() (PriceVolume, bool) {
	if len(b.Bids) == 0 {
		return PriceVolume{}, false
	}

	return b.Bids[0], true
}

func (b *OrderBook) BestAsk() (PriceVolume, bool) {
	if len(b.Asks) == 0 {
		return PriceVolume{}, false
	}

	return b.Asks[0], true
}

func (b *OrderBook) IsValid() (bool, error) {
	bid, hasBid := b.BestBid()
	ask, hasAsk := b.BestAsk()

	if !hasBid {
		return false, errors.New("empty bids")
	}

	if !hasAsk {
		return false, errors.New("empty asks")
	}

	return bid.Price < ask.Price, fmt.Errorf("bid price %f > ask price %f", bid.Price.Float64(), ask.Price.Float64())
}

func (b *OrderBook) PriceVolumesBySide(side SideType) PriceVolumeSlice {
	switch side {

	case SideTypeBuy:
		return b.Bids

	case SideTypeSell:
		return b.Asks
	}

	return nil
}

func (b *OrderBook) Copy() (book OrderBook) {
	book = *b
	book.Bids = b.Bids.Copy()
	book.Asks = b.Asks.Copy()
	return book
}

func (b *OrderBook) updateAsks(pvs PriceVolumeSlice) {
	for _, pv := range pvs {
		if pv.Volume == 0 {
			b.Asks = b.Asks.Remove(pv.Price, false)
		} else {
			b.Asks = b.Asks.Upsert(pv, false)
		}
	}

	b.EmitAsksChange(b.Asks)
}

func (b *OrderBook) updateBids(pvs PriceVolumeSlice) {
	for _, pv := range pvs {
		if pv.Volume == 0 {
			b.Bids = b.Bids.Remove(pv.Price, true)
		} else {
			b.Bids = b.Bids.Upsert(pv, true)
		}
	}

	b.EmitBidsChange(b.Bids)
}

func (b *OrderBook) update(book OrderBook) {
	b.updateBids(book.Bids)
	b.updateAsks(book.Asks)
}

func (b *OrderBook) Reset() {
	b.Bids = nil
	b.Asks = nil
}

func (b *OrderBook) Load(book OrderBook) {
	b.Reset()
	b.update(book)
	b.EmitLoad(b)
}

func (b *OrderBook) Update(book OrderBook) {
	b.update(book)
	b.EmitUpdate(b)
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

type MutexOrderBook struct {
	sync.Mutex

	*OrderBook
}

func NewMutexOrderBook(symbol string) *MutexOrderBook {
	return &MutexOrderBook{
		OrderBook: &OrderBook{Symbol: symbol},
	}
}

func (b *MutexOrderBook) Load(book OrderBook) {
	b.Lock()
	defer b.Unlock()

	b.Reset()
	b.update(book)
	b.EmitLoad(b.OrderBook)
}

func (b *MutexOrderBook) Get() OrderBook {
	return b.OrderBook.Copy()
}

func (b *MutexOrderBook) Update(book OrderBook) {
	b.Lock()
	defer b.Unlock()

	b.update(book)
	b.EmitUpdate(b.OrderBook)
}

// StreamOrderBook receives streaming data from websocket connection and
// update the order book with mutex lock, so you can safely access it.
type StreamOrderBook struct {
	*MutexOrderBook

	C sigchan.Chan
}

func NewStreamBook(symbol string) *StreamOrderBook {
	return &StreamOrderBook{
		MutexOrderBook: NewMutexOrderBook(symbol),
		C:              sigchan.New(60),
	}
}

func (sb *StreamOrderBook) BindStream(stream Stream) {
	stream.OnBookSnapshot(func(book OrderBook) {
		if sb.Symbol != book.Symbol {
			return
		}

		sb.Load(book)
		sb.C.Emit()
	})

	stream.OnBookUpdate(func(book OrderBook) {
		if sb.Symbol != book.Symbol {
			return
		}

		sb.Update(book)
		sb.C.Emit()
	})
}
