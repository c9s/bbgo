package types

import (
	"fmt"
	"os"
	"strconv"
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
	Copy() OrderBook
	CopyDepth(depth int) OrderBook
	SideBook(sideType SideType) PriceVolumeSlice
	IsValid() (bool, error)
}

type MutexOrderBook struct {
	sync.Mutex

	Symbol    string
	OrderBook OrderBook
}

func NewMutexOrderBook(symbol string) *MutexOrderBook {
	var book OrderBook = NewSliceOrderBook(symbol)

	if v, _ := strconv.ParseBool(os.Getenv("ENABLE_RBT_ORDERBOOK")); v {
		book = NewRBOrderBook(symbol)
	}

	return &MutexOrderBook{
		Symbol:    symbol,
		OrderBook: book,
	}
}

func (b *MutexOrderBook) Load(book SliceOrderBook) {
	b.Lock()
	b.OrderBook.Load(book)
	b.Unlock()
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

func (b *MutexOrderBook) Copy() OrderBook {
	b.Lock()
	defer b.Unlock()
	return b.OrderBook.Copy()
}

func (b *MutexOrderBook) Update(update SliceOrderBook) {
	b.Lock()
	b.OrderBook.Update(update)
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
		if sb.MutexOrderBook.Symbol != book.Symbol {
			return
		}

		sb.Load(book)
		sb.C.Emit()
	})

	stream.OnBookUpdate(func(book SliceOrderBook) {
		if sb.MutexOrderBook.Symbol != book.Symbol {
			return
		}

		sb.Update(book)
		sb.C.Emit()
	})
}
