package types

import (
	"fmt"
	"sync"

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

type OrderBook interface {
	Spread() (fixedpoint.Value, bool)
	BestAsk() (PriceVolume, bool)
	BestBid() (PriceVolume, bool)
	Reset()
	Load(book SliceOrderBook)
	Update(book SliceOrderBook)
}

type MutexOrderBook struct {
	sync.Mutex

	*SliceOrderBook
}

func NewMutexOrderBook(symbol string) *MutexOrderBook {
	return &MutexOrderBook{
		SliceOrderBook: &SliceOrderBook{Symbol: symbol},
	}
}

func (b *MutexOrderBook) Load(book SliceOrderBook) {
	b.Lock()
	b.SliceOrderBook.Load(book)
	b.Unlock()
}

func (b *MutexOrderBook) Reset() {
	b.Lock()
	b.SliceOrderBook.Reset()
	b.Unlock()
}

func (b *MutexOrderBook) CopyDepth(depth int) SliceOrderBook {
	b.Lock()
	defer b.Unlock()
	return b.SliceOrderBook.CopyDepth(depth)
}

func (b *MutexOrderBook) Copy() SliceOrderBook {
	b.Lock()
	defer b.Unlock()
	return b.SliceOrderBook.Copy()
}

func (b *MutexOrderBook) Update(update SliceOrderBook) {
	b.Lock()
	b.SliceOrderBook.Update(update)
	b.Unlock()
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
	stream.OnBookSnapshot(func(book SliceOrderBook) {
		if sb.Symbol != book.Symbol {
			return
		}

		sb.Load(book)
		sb.C.Emit()
	})

	stream.OnBookUpdate(func(book SliceOrderBook) {
		if sb.Symbol != book.Symbol {
			return
		}

		sb.Update(book)
		sb.C.Emit()
	})
}
