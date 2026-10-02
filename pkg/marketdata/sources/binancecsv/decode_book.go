package binancecsv

import (
	"fmt"
	"strconv"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/archive"
	"github.com/c9s/bbgo/pkg/types"
)

var bookTickerColumns = []string{
	"update_id", "best_bid_price", "best_bid_qty",
	"best_ask_price", "best_ask_qty", "transaction_time", "event_time",
}

type bookTickerDecoder struct{ cfg Config }

func newBookTickerDecoder(cfg Config) RecordDecoder { return &bookTickerDecoder{cfg: cfg} }

func (d *bookTickerDecoder) Columns() []string { return bookTickerColumns }

func (d *bookTickerDecoder) Decode(
	dst []marketdata.Event, record []string, meta RecordMeta,
) ([]marketdata.Event, error) {
	if len(record) < 7 {
		return dst, fmt.Errorf("binancecsv: bookTicker record has %d fields, want at least 7", len(record))
	}

	updateID, err := strconv.ParseInt(record[meta.column("update_id", 0)], 10, 64)
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad update_id: %w", err)
	}

	bidPrice, err := fixedpoint.NewFromString(record[meta.column("best_bid_price", 1)])
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad best_bid_price: %w", err)
	}
	bidQty, err := fixedpoint.NewFromString(record[meta.column("best_bid_qty", 2)])
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad best_bid_qty: %w", err)
	}
	askPrice, err := fixedpoint.NewFromString(record[meta.column("best_ask_price", 3)])
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad best_ask_price: %w", err)
	}
	askQty, err := fixedpoint.NewFromString(record[meta.column("best_ask_qty", 4)])
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad best_ask_qty: %w", err)
	}

	// Order by transaction_time, the moment the book changed in the matching
	// engine, not event_time, the moment Binance published it. Under simulated
	// time only the former is meaningful; the difference between them is
	// publication latency, which is kept so it can be modelled later.
	txNs, err := parseEventTimeNano(record[meta.column("transaction_time", 5)])
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad transaction_time: %w", err)
	}
	evtNs, err := parseEventTimeNano(record[meta.column("event_time", 6)])
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad event_time: %w", err)
	}

	ticker := &marketdata.BookTicker{
		BookTicker: types.BookTicker{
			Symbol:   meta.Symbol,
			Buy:      bidPrice,
			BuySize:  bidQty,
			Sell:     askPrice,
			SellSize: askQty,
		},
		UpdateID:        updateID,
		TransactionTime: types.Time(nanoTime(txNs)),
		EventTime:       types.Time(nanoTime(evtNs)),
	}

	dst = append(dst, marketdata.Event{
		Type:     marketdata.EventTypeBookTicker,
		Exchange: meta.Exchange,
		Symbol:   meta.Symbol,
		Key: marketdata.OrderKey{
			TimeNs: txNs,
			Rank:   marketdata.RankBookTicker,
			Seq:    uint64(updateID),
		},
		BookTicker: ticker,
	})

	if d.cfg.SynthesizeBookFromBookTicker {
		dst = append(dst, marketdata.Event{
			Type:     marketdata.EventTypeBookSnapshot,
			Flags:    marketdata.FlagSynthetic | marketdata.FlagPartialDepth,
			Exchange: meta.Exchange,
			Symbol:   meta.Symbol,
			Key: marketdata.OrderKey{
				TimeNs: txNs,
				Rank:   marketdata.RankBookSnapshot,
				Seq:    uint64(updateID),
			},
			Book: &types.SliceOrderBook{
				Symbol:       meta.Symbol,
				Bids:         types.PriceVolumeSlice{{Price: bidPrice, Volume: bidQty}},
				Asks:         types.PriceVolumeSlice{{Price: askPrice, Volume: askQty}},
				Time:         nanoTime(txNs),
				LastUpdateId: updateID,
			},
		})
	}

	return dst, nil
}

var bookDepthColumns = []string{"timestamp", "percentage", "depth", "notional"}

// bookDepthDecoder decodes the bookDepth dataset.
//
// This dataset is aggregate notional resting within a percentage band of mid,
// sampled about once a minute. It has no price levels, so it is emitted as
// EventTypeDepthBand rather than as a book, and it must never reach a
// BookState. Giving it its own event type rather than folding it into metrics
// is what makes that visible in the type system.
type bookDepthDecoder struct{ cfg Config }

func newBookDepthDecoder(cfg Config) RecordDecoder { return &bookDepthDecoder{cfg: cfg} }

func (d *bookDepthDecoder) Columns() []string { return bookDepthColumns }

func (d *bookDepthDecoder) Decode(
	dst []marketdata.Event, record []string, meta RecordMeta,
) ([]marketdata.Event, error) {
	if len(record) < 4 {
		return dst, fmt.Errorf("binancecsv: bookDepth record has %d fields, want at least 4", len(record))
	}

	// This dataset carries a formatted timestamp rather than an epoch.
	ts, err := archive.ParseDateTime(record[meta.column("timestamp", 0)])
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad bookDepth timestamp: %w", err)
	}

	percentage, err := fixedpoint.NewFromString(record[meta.column("percentage", 1)])
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad percentage: %w", err)
	}
	depth, err := fixedpoint.NewFromString(record[meta.column("depth", 2)])
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad depth: %w", err)
	}
	notional, err := fixedpoint.NewFromString(record[meta.column("notional", 3)])
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad notional: %w", err)
	}

	return append(dst, marketdata.Event{
		Type:     marketdata.EventTypeDepthBand,
		Exchange: meta.Exchange,
		Symbol:   meta.Symbol,
		Key: marketdata.OrderKey{
			TimeNs: ts.UnixNano(),
			Rank:   marketdata.RankDepthBand,
		},
		DepthBand: &marketdata.DepthBand{
			Symbol:     meta.Symbol,
			Time:       types.Time(ts),
			Percentage: percentage,
			Depth:      depth,
			Notional:   notional,
		},
	}), nil
}
