package replay

import (
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/types"
)

// Recorder captures a live market data stream into a replayable file.
//
// It consumes types.Stream callbacks rather than an exchange-specific feed, so
// it works with every adapter bbgo supports, and it binds to exactly the
// callbacks the backtest engine will later consume — which is what makes a
// recording a faithful input for replay rather than an approximation.
type Recorder struct {
	writer   *Writer
	exchange types.ExchangeName

	// Dropped counts events that could not be written. A recorder must not take
	// down a live session because a disk filled up, so failures are counted and
	// logged rather than propagated.
	dropped int64

	logger *log.Entry
}

// NewRecorder returns a Recorder writing through w.
func NewRecorder(w *Writer, exchange types.ExchangeName) *Recorder {
	return &Recorder{
		writer:   w,
		exchange: exchange,
		logger:   log.WithField("component", "marketdata.recorder"),
	}
}

// Dropped returns how many events failed to be written.
func (r *Recorder) Dropped() int64 { return r.dropped }

// BindStream subscribes to the market data callbacks worth recording.
//
// Call it before the stream connects: StandardStream does not lock its callback
// slices, so registration has to finish before emission starts.
func (r *Recorder) BindStream(stream types.Stream) {
	stream.OnBookSnapshot(func(book types.SliceOrderBook) {
		r.record(&marketdata.Event{
			Type:     marketdata.EventTypeBookSnapshot,
			Exchange: r.exchange,
			Symbol:   book.Symbol,
			Key: marketdata.OrderKey{
				TimeNs: bookTimeNs(book),
				Rank:   marketdata.RankBookSnapshot,
				Seq:    uint64(book.LastUpdateId),
			},
			Book: &book,
		})
	})

	stream.OnBookUpdate(func(book types.SliceOrderBook) {
		r.record(&marketdata.Event{
			Type:     marketdata.EventTypeBookUpdate,
			Exchange: r.exchange,
			Symbol:   book.Symbol,
			Key: marketdata.OrderKey{
				TimeNs: bookTimeNs(book),
				Rank:   marketdata.RankBookUpdate,
				Seq:    uint64(book.LastUpdateId),
			},
			Book: &book,
		})
	})

	stream.OnMarketTrade(func(trade types.Trade) {
		r.record(&marketdata.Event{
			Type:     marketdata.EventTypeTrade,
			Exchange: r.exchange,
			Symbol:   trade.Symbol,
			Key: marketdata.OrderKey{
				TimeNs: trade.Time.Time().UnixNano(),
				Rank:   marketdata.RankTrade,
				Seq:    trade.ID,
			},
			Trade: &trade,
		})
	})

	stream.OnAggTrade(func(trade types.Trade) {
		r.record(&marketdata.Event{
			Type:     marketdata.EventTypeTrade,
			Flags:    marketdata.FlagAggregated,
			Exchange: r.exchange,
			Symbol:   trade.Symbol,
			Key: marketdata.OrderKey{
				TimeNs: trade.Time.Time().UnixNano(),
				Rank:   marketdata.RankTrade,
				Seq:    trade.ID,
			},
			Trade: &trade,
		})
	})

	stream.OnKLineClosed(func(kline types.KLine) {
		r.record(&marketdata.Event{
			Type:     marketdata.EventTypeKLine,
			Exchange: r.exchange,
			Symbol:   kline.Symbol,
			Key: marketdata.OrderKey{
				TimeNs: kline.EndTime.Time().UnixNano(),
				Rank:   marketdata.KLineRank(kline.Interval),
			},
			KLine: &kline,
		})
	})

	stream.OnBookTickerUpdate(func(ticker types.BookTicker) {
		// types.BookTicker has no timestamp — the field is commented out in
		// pkg/types/bookticker.go — so a recording cannot recover the venue's
		// own time here and the arrival time is the best available. That is why
		// marketdata.BookTicker carries explicit timestamps: a source that has
		// them should not lose them.
		now := nowNs()
		r.record(&marketdata.Event{
			Type:     marketdata.EventTypeBookTicker,
			Exchange: r.exchange,
			Symbol:   ticker.Symbol,
			Key: marketdata.OrderKey{
				TimeNs: now,
				Rank:   marketdata.RankBookTicker,
			},
			BookTicker: &marketdata.BookTicker{
				BookTicker:      ticker,
				TransactionTime: types.Time(nanoTime(now)),
				EventTime:       types.Time(nanoTime(now)),
			},
		})
	})
}

func (r *Recorder) record(ev *marketdata.Event) {
	if err := r.writer.WriteEvent(ev); err != nil {
		r.dropped++
		r.logger.WithError(err).Warnf("dropped a %s event for %s", ev.Type, ev.Symbol)
	}
}

// bookTimeNs prefers the venue's own time and falls back to arrival time. Some
// adapters leave SliceOrderBook.Time empty, and an event with a zero time would
// sort before everything else in a merge.
func bookTimeNs(book types.SliceOrderBook) int64 {
	if !book.Time.IsZero() {
		return book.Time.UnixNano()
	}
	return nowNs()
}
