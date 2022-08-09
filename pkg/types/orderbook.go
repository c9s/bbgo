package types

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/sigchan"
)

type OrderBook interface {
	Spread() (fixedpoint.Value, bool)
	BestAsk() (PriceVolume, bool)
	BestBid() (PriceVolume, bool)
	LastUpdateTime() time.Time
	Reset()
	Load(book SliceOrderBook)
	Update(book SliceOrderBook)
	Copy() OrderBook
	SideBook(sideType SideType) PriceVolumeSlice
	CopyDepth(depth int) OrderBook
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

func (b *MutexOrderBook) IsValid() (ok bool, err error) {
	b.Lock()
	ok, err = b.OrderBook.IsValid()
	b.Unlock()
	return ok, err
}

func (b *MutexOrderBook) LastUpdateTime() time.Time {
	b.Lock()
	t := b.OrderBook.LastUpdateTime()
	b.Unlock()
	return t
}

func (b *MutexOrderBook) BestBidAndAsk() (bid, ask PriceVolume, ok bool) {
	var ok1, ok2 bool
	b.Lock()
	bid, ok1 = b.OrderBook.BestBid()
	ask, ok2 = b.OrderBook.BestAsk()
	b.Unlock()
	ok = ok1 && ok2
	return bid, ask, ok
}

func (b *MutexOrderBook) BestBid() (pv PriceVolume, ok bool) {
	b.Lock()
	pv, ok = b.OrderBook.BestBid()
	b.Unlock()
	return pv, ok
}

func (b *MutexOrderBook) BestAsk() (pv PriceVolume, ok bool) {
	b.Lock()
	pv, ok = b.OrderBook.BestAsk()
	b.Unlock()
	return pv, ok
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
	book := b.OrderBook.CopyDepth(depth)
	b.Unlock()
	return book
}

func (b *MutexOrderBook) Copy() OrderBook {
	b.Lock()
	book := b.OrderBook.Copy()
	b.Unlock()
	return book
}

func (b *MutexOrderBook) Update(update SliceOrderBook) {
	b.Lock()
	b.OrderBook.Update(update)
	b.Unlock()
}

//go:generate callbackgen -type StreamOrderBook
// StreamOrderBook receives streaming data from websocket connection and
// update the order book with mutex lock, so you can safely access it.
type StreamOrderBook struct {
	*MutexOrderBook

	C sigchan.Chan

	updateCallbacks   []func(update SliceOrderBook)
	snapshotCallbacks []func(snapshot SliceOrderBook)
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
		sb.EmitSnapshot(book)
		sb.C.Emit()
	})

	stream.OnBookUpdate(func(book SliceOrderBook) {
		if sb.MutexOrderBook.Symbol != book.Symbol {
			return
		}

		sb.Update(book)
		sb.EmitUpdate(book)
		sb.C.Emit()
	})
}
