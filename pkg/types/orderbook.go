package types

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

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
		labels := prometheus.Labels{"symbol": sb.Symbol, "exchange": exchangeName}
		streamOrderBookBestAskPriceMetrics.With(labels).Set(bestAsk.Price.Float64())
		streamOrderBookBestBidPriceMetrics.With(labels).Set(bestBid.Price.Float64())
		streamOrderBookBestAskVolumeMetrics.With(labels).Set(bestAsk.Volume.Float64())
		streamOrderBookBestBidVolumeMetrics.With(labels).Set(bestBid.Volume.Float64())
		streamOrderBookUpdateTimeMetrics.With(labels).Set(float64(t.UnixMilli()))
	}
}

func (sb *StreamOrderBook) BindStream(stream Stream) {
	stream.OnDisconnect(func() {
		sb.Reset()
	})

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

func (sb *StreamOrderBook) DebugBindStream(stream Stream, log logrus.FieldLogger) {
	stream.OnDisconnect(func() {
		log.Infof("stream disconnected, resetting order book %s on %s", sb.Symbol, sb.Exchange)
	})

	stream.OnBookSnapshot(func(snapshot SliceOrderBook) {
		log.Infof("received order book snapshot for %s on %s: %+v", sb.Symbol, sb.Exchange, snapshot)
	})

	stream.OnBookUpdate(func(update SliceOrderBook) {
		log.Infof("received order book update for %s on %s: %+v", sb.Symbol, sb.Exchange, update)
	})
}

func (sb *StreamOrderBook) BindUpdate(updateFunc func(book *StreamOrderBook, update SliceOrderBook)) {
	sb.OnUpdate(func(update_ SliceOrderBook) {
		updateFunc(sb, update_)
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

// DepthBook wraps a StreamOrderBook and provides price calculation at a specific quote depth.
type DepthBook struct {
	Source *StreamOrderBook
}

// NewDepthBook creates a DepthBook from a StreamOrderBook and a quote depth.
func NewDepthBook(source *StreamOrderBook) *DepthBook {
	return &DepthBook{Source: source}
}

// PriceAtQuoteDepth returns the average price at the specified quote depth for the given side.
// If the depth is zero or negative, returns zero.
func (db *DepthBook) PriceAtQuoteDepth(side SideType, quoteDepth fixedpoint.Value) fixedpoint.Value {
	if db.Source == nil || quoteDepth.Sign() <= 0 {
		return fixedpoint.Zero
	}

	pvs := db.Source.SideBook(side)
	if len(pvs) == 0 {
		return fixedpoint.Zero
	}

	sumQty := fixedpoint.Zero
	sumQuote := fixedpoint.Zero
	for _, pv := range pvs {
		amount := pv.Price.Mul(pv.Volume)
		if sumQuote.Add(amount).Compare(quoteDepth) >= 0 {
			remain := quoteDepth.Sub(sumQuote)
			if pv.Price.Sign() > 0 {
				partialQty := remain.Div(pv.Price)
				sumQty = sumQty.Add(partialQty)
				sumQuote = sumQuote.Add(partialQty.Mul(pv.Price))
			}
			break
		}
		sumQty = sumQty.Add(pv.Volume)
		sumQuote = sumQuote.Add(amount)
	}
	if sumQty.Sign() == 0 {
		return fixedpoint.Zero
	}
	return sumQuote.Div(sumQty)
}

// BestBidAndAskAtQuoteDepth returns the bid and ask price at the specified quote depth.
func (db *DepthBook) BestBidAndAskAtQuoteDepth(depth fixedpoint.Value) (bid, ask fixedpoint.Value) {
	bid = db.PriceAtQuoteDepth(SideTypeBuy, depth)
	ask = db.PriceAtQuoteDepth(SideTypeSell, depth)
	return bid, ask
}

func (db *DepthBook) BestBidAndAskAtDepth(depth fixedpoint.Value) (bid, ask fixedpoint.Value) {
	bid = db.PriceAtDepth(SideTypeBuy, depth)
	ask = db.PriceAtDepth(SideTypeSell, depth)
	return bid, ask
}

// PriceAtDepth returns the average price at which the cumulative base volume
// reaches the specified baseVolume for the given side.
// This is similar to PriceAtQuoteDepth but accumulates based on base quantity.
func (db *DepthBook) PriceAtDepth(side SideType, baseVolume fixedpoint.Value) fixedpoint.Value {
	if db.Source == nil || baseVolume.Sign() <= 0 {
		return fixedpoint.Zero
	}

	pvs := db.Source.SideBook(side)
	if len(pvs) == 0 {
		return fixedpoint.Zero
	}

	sumBase := fixedpoint.Zero
	sumPrice := fixedpoint.Zero
	for _, pv := range pvs {
		if sumBase.Add(pv.Volume).Compare(baseVolume) >= 0 {
			remain := baseVolume.Sub(sumBase)
			sumPrice = sumPrice.Add(remain.Mul(pv.Price))
			sumBase = baseVolume
			break
		}
		sumBase = sumBase.Add(pv.Volume)
		sumPrice = sumPrice.Add(pv.Volume.Mul(pv.Price))
	}

	if sumBase.IsZero() {
		return fixedpoint.Zero
	}
	return sumPrice.Div(sumBase)
}
