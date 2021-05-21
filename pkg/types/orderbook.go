package types

import (
	"fmt"
	"strings"
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

//go:generate callbackgen -type RBOrderBook
type RBOrderBook struct {
	Symbol string
	Bids   *RBTree
	Asks   *RBTree

	loadCallbacks   []func(book *RBOrderBook)
	updateCallbacks []func(book *RBOrderBook)
}

func NewRBOrderBook(symbol string) *RBOrderBook {
	return &RBOrderBook{
		Symbol: symbol,
		Bids:   NewRBTree(),
		Asks:   NewRBTree(),
	}
}

func (b *RBOrderBook) BestBid() (PriceVolume, bool) {
	return PriceVolume{}, true
}

func (b *RBOrderBook) BestAsk() (PriceVolume, bool) {
	return PriceVolume{}, true
}

//go:generate callbackgen -type OrderBook
type OrderBook struct {
	Symbol string
	Bids   PriceVolumeSlice
	Asks   PriceVolumeSlice

	loadCallbacks   []func(book *OrderBook)
	updateCallbacks []func(book *OrderBook)
}

func (b *OrderBook) Spread() (fixedpoint.Value, bool) {
	bestBid, ok := b.BestBid()
	if !ok {
		return 0, false
	}

	bestAsk, ok := b.BestAsk()
	if !ok {
		return 0, false
	}

	return bestAsk.Price - bestBid.Price, true
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

	if bid.Price > ask.Price {
		return false, fmt.Errorf("bid price %f > ask price %f", bid.Price.Float64(), ask.Price.Float64())
	}

	return true, nil
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

func (b *OrderBook) CopyDepth(depth int) (book OrderBook) {
	book = *b
	book.Bids = book.Bids.CopyDepth(depth)
	book.Asks = book.Asks.CopyDepth(depth)
	return book
}

func (b *OrderBook) Copy() (book OrderBook) {
	book = *b
	book.Bids = book.Bids.Copy()
	book.Asks = book.Asks.Copy()
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
}

func (b *OrderBook) updateBids(pvs PriceVolumeSlice) {
	for _, pv := range pvs {
		if pv.Volume == 0 {
			b.Bids = b.Bids.Remove(pv.Price, true)
		} else {
			b.Bids = b.Bids.Upsert(pv, true)
		}
	}
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
	fmt.Printf(b.String())
}

func (b *OrderBook) String() string {
	sb := strings.Builder{}

	sb.WriteString("BOOK ")
	sb.WriteString(b.Symbol)
	sb.WriteString("\n")

	if len(b.Asks) > 0 {
		sb.WriteString("ASKS:\n")
		for i := len(b.Asks) - 1; i >= 0; i-- {
			sb.WriteString("- ASK: ")
			sb.WriteString(b.Asks[i].String())
			sb.WriteString("\n")
		}
	}

	if len(b.Bids) > 0 {
		sb.WriteString("BIDS:\n")
		for _, bid := range b.Bids {
			sb.WriteString("- BID: ")
			sb.WriteString(bid.String())
			sb.WriteString("\n")
		}
	}

	return sb.String()
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

	b.OrderBook.Reset()
	b.OrderBook.update(book)
	b.EmitLoad(b.OrderBook)
}

func (b *MutexOrderBook) Reset() {
	b.Lock()
	b.OrderBook.Reset()
	b.Unlock()
}

func (b *MutexOrderBook) CopyDepth(depth int) OrderBook {
	b.Lock()
	defer b.Unlock()
	return b.OrderBook.CopyDepth(depth)
}

func (b *MutexOrderBook) Get() OrderBook {
	b.Lock()
	defer b.Unlock()
	return b.OrderBook.Copy()
}

func (b *MutexOrderBook) Update(update OrderBook) {
	b.Lock()
	defer b.Unlock()

	b.OrderBook.update(update)
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
