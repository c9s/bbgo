package types

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
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

	Symbol   string
	Exchange ExchangeName

	orderBook OrderBook
}

func NewMutexOrderBook(symbol string, exchangeName ExchangeName) *MutexOrderBook {
	var book OrderBook = NewSliceOrderBook(symbol)

	if v, _ := strconv.ParseBool(os.Getenv("ENABLE_RBT_ORDERBOOK")); v {
		book = NewRBOrderBook(symbol)
	}

	return &MutexOrderBook{
		Symbol:    symbol,
		Exchange:  exchangeName,
		orderBook: book,
	}
}

func (b *MutexOrderBook) IsValid() (ok bool, err error) {
	b.Lock()
	ok, err = b.orderBook.IsValid()
	b.Unlock()
	return ok, err
}

func (b *MutexOrderBook) SideBook(sideType SideType) PriceVolumeSlice {
	b.Lock()
	sideBook := b.orderBook.SideBook(sideType)
	b.Unlock()
	return sideBook
}

func (b *MutexOrderBook) LastUpdateTime() time.Time {
	b.Lock()
	t := b.orderBook.LastUpdateTime()
	b.Unlock()
	return t
}

func (b *MutexOrderBook) BestBidAndAsk() (bid, ask PriceVolume, ok bool) {
	var ok1, ok2 bool
	b.Lock()
	bid, ok1 = b.orderBook.BestBid()
	ask, ok2 = b.orderBook.BestAsk()
	b.Unlock()
	ok = ok1 && ok2
	return bid, ask, ok
}

func (b *MutexOrderBook) BestBid() (pv PriceVolume, ok bool) {
	b.Lock()
	pv, ok = b.orderBook.BestBid()
	b.Unlock()
	return pv, ok
}

func (b *MutexOrderBook) BestAsk() (pv PriceVolume, ok bool) {
	b.Lock()
	pv, ok = b.orderBook.BestAsk()
	b.Unlock()
	return pv, ok
}

func (b *MutexOrderBook) Load(book SliceOrderBook) {
	b.Lock()
	b.orderBook.Load(book)
	b.Unlock()
}

func (b *MutexOrderBook) Reset() {
	b.Lock()
	b.orderBook.Reset()
	b.Unlock()
}

func (b *MutexOrderBook) CopyDepth(depth int) (ob OrderBook) {
	b.Lock()
	ob = b.orderBook.CopyDepth(depth)
	b.Unlock()
	return ob
}

func (b *MutexOrderBook) Copy() (ob OrderBook) {
	b.Lock()
	ob = b.orderBook.Copy()
	b.Unlock()

	return ob
}

func (b *MutexOrderBook) Update(update SliceOrderBook) {
	b.Lock()
	b.orderBook.Update(update)
	b.Unlock()
}

type BookSignalType int

const (
	BookSignalSnapshot BookSignalType = 1
	BookSignalUpdate   BookSignalType = 2
)

type BookSignal struct {
	Type BookSignalType
	Time time.Time
}

var streamOrderBookBestBidPriceMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_stream_order_book_best_bid_price",
		Help: "",
	}, []string{"symbol", "exchange"})

var streamOrderBookBestAskPriceMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_stream_order_book_best_ask_price",
		Help: "",
	}, []string{"symbol", "exchange"})

var streamOrderBookBestBidVolumeMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_stream_order_book_best_bid_volume",
		Help: "",
	}, []string{"symbol", "exchange"})

var streamOrderBookBestAskVolumeMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_stream_order_book_best_ask_volume",
		Help: "",
	}, []string{"symbol", "exchange"})

var streamOrderBookUpdateTimeMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_stream_order_book_update_time_milliseconds",
		Help: "",
	}, []string{"symbol", "exchange"})

func init() {
	prometheus.MustRegister(
		streamOrderBookBestBidPriceMetrics,
		streamOrderBookBestAskPriceMetrics,
		streamOrderBookBestBidVolumeMetrics,
		streamOrderBookBestAskVolumeMetrics,
		streamOrderBookUpdateTimeMetrics,
	)
}

// StreamOrderBook receives streaming data from websocket connection and
// update the order book with mutex lock, so you can safely access it.
//
//go:generate callbackgen -type StreamOrderBook
type StreamOrderBook struct {
	*MutexOrderBook

	C chan *BookSignal

	updateCallbacks   []func(update SliceOrderBook)
	snapshotCallbacks []func(snapshot SliceOrderBook)
}

func NewStreamBook(symbol string, exchangeName ExchangeName) *StreamOrderBook {
	return &StreamOrderBook{
		MutexOrderBook: NewMutexOrderBook(symbol, exchangeName),
		C:              make(chan *BookSignal, 1),
	}
}

func (sb *StreamOrderBook) updateMetrics(t time.Time) {
	bestBid, bestAsk, ok := sb.BestBidAndAsk()
	if ok {
		exchangeName := string(sb.Exchange)
		streamOrderBookBestAskPriceMetrics.WithLabelValues(sb.Symbol, exchangeName).Set(bestAsk.Price.Float64())
		streamOrderBookBestBidPriceMetrics.WithLabelValues(sb.Symbol, exchangeName).Set(bestBid.Price.Float64())
		streamOrderBookBestAskVolumeMetrics.WithLabelValues(sb.Symbol, exchangeName).Set(bestAsk.Volume.Float64())
		streamOrderBookBestBidVolumeMetrics.WithLabelValues(sb.Symbol, exchangeName).Set(bestBid.Volume.Float64())
		streamOrderBookUpdateTimeMetrics.WithLabelValues(sb.Symbol, exchangeName).Set(float64(t.UnixMilli()))
	}
}

func (sb *StreamOrderBook) BindStream(stream Stream) {
	stream.OnBookSnapshot(func(book SliceOrderBook) {
		if sb.MutexOrderBook.Symbol != book.Symbol {
			return
		}

		sb.Load(book)
		sb.EmitSnapshot(book)
		sb.emitChange(BookSignalSnapshot, book.Time)
		sb.updateMetrics(book.Time)
	})

	stream.OnBookUpdate(func(book SliceOrderBook) {
		if sb.MutexOrderBook.Symbol != book.Symbol {
			return
		}

		sb.Update(book)
		sb.EmitUpdate(book)
		sb.emitChange(BookSignalUpdate, book.Time)
		sb.updateMetrics(book.Time)
	})
}

func (sb *StreamOrderBook) emitChange(signalType BookSignalType, bookTime time.Time) {
	select {
	case sb.C <- &BookSignal{Type: signalType, Time: defaultTime(bookTime, time.Now)}:
	default:
	}
}

func defaultTime(a time.Time, b func() time.Time) time.Time {
	if a.IsZero() {
		return b()
	}

	return a
}
