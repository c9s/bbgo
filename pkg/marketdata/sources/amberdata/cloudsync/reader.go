package cloudsync

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/parquet-go/parquet-go"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/types"
)

// readBatch is how many rows are pulled from the file at a time. A day of
// Binance order book updates is a few hundred thousand rows of up to a few
// hundred levels each, so the file is streamed rather than loaded.
const readBatch = 512

// Feature identifies which CloudSync product a file holds, which determines how
// its rows decode.
type Feature string

const (
	// FeatureOrderBookUpdates holds incremental changes: a level with volume
	// zero means removal.
	FeatureOrderBookUpdates Feature = "order-book-updates"

	// FeatureOrderBookSnapshots holds complete book states.
	FeatureOrderBookSnapshots Feature = "order-book-snapshots"

	FeatureTrades Feature = "trades"
)

// BookReader streams book events out of a Parquet file.
//
// One logical book change is stored as up to two rows, one per side, sharing a
// timestamp and sequence. The reader pairs them, so a consumer sees one event
// with both sides rather than two half-events that would each look like the
// removal of the other side.
type BookReader struct {
	file    *os.File
	reader  *parquet.GenericReader[bookRow]
	feature Feature

	exchange types.ExchangeName
	symbol   string

	buf     []bookRow
	bufLen  int
	bufPos  int
	pending *bookRow
	eof     bool
}

// OpenBook opens a book file. The symbol and exchange override what the rows
// carry, for the case where bbgo's naming differs from the venue's.
func OpenBook(
	path string, feature Feature, exchange types.ExchangeName, symbol string,
) (*BookReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &BookReader{
		file:     f,
		reader:   parquet.NewGenericReader[bookRow](f),
		feature:  feature,
		exchange: exchange,
		symbol:   symbol,
		buf:      make([]bookRow, readBatch),
	}, nil
}

// Read returns the next book event, or io.EOF.
func (r *BookReader) Read() (marketdata.Event, error) {
	row, err := r.nextRow()
	if err != nil {
		return marketdata.Event{}, err
	}

	// Look ahead one row: if it belongs to the same change, it is the other side.
	var other *bookRow
	next, err := r.nextRow()
	switch {
	case err == nil && sameChange(row, next):
		copied := next
		other = &copied
	case err == nil:
		copied := next
		r.pending = &copied
	case err == io.EOF:
		r.eof = true
	default:
		return marketdata.Event{}, err
	}

	return r.toEvent(row, other), nil
}

// ReadAll drains the reader.
func (r *BookReader) ReadAll() ([]marketdata.Event, error) {
	var out []marketdata.Event
	for {
		ev, err := r.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, ev)
	}
}

func (r *BookReader) nextRow() (bookRow, error) {
	if r.pending != nil {
		row := *r.pending
		r.pending = nil
		return row, nil
	}

	for r.bufPos >= r.bufLen {
		if r.eof {
			return bookRow{}, io.EOF
		}

		n, err := r.reader.Read(r.buf)
		if n == 0 {
			if err == nil {
				err = io.EOF
			}
			r.eof = true
			return bookRow{}, err
		}
		if err == io.EOF {
			r.eof = true
		} else if err != nil {
			return bookRow{}, err
		}

		r.bufLen, r.bufPos = n, 0
	}

	row := r.buf[r.bufPos]
	r.bufPos++
	return row, nil
}

// sameChange reports whether two rows are the two sides of one book change.
func sameChange(a, b bookRow) bool {
	return a.Sequence == b.Sequence &&
		a.ExchangeTimestamp == b.ExchangeTimestamp &&
		a.ExchangeTimestampNanoseconds == b.ExchangeTimestampNanoseconds &&
		a.IsBid != b.IsBid
}

func (r *BookReader) toEvent(row bookRow, other *bookRow) marketdata.Event {
	timeNs := eventTimeNano(row.ExchangeTimestamp, row.ExchangeTimestampNanoseconds)

	evType := marketdata.EventTypeBookUpdate
	rank := marketdata.RankBookUpdate
	if r.feature == FeatureOrderBookSnapshots {
		evType = marketdata.EventTypeBookSnapshot
		rank = marketdata.RankBookSnapshot
	}

	book := &types.SliceOrderBook{
		Symbol:       r.symbolOr(row.Instrument),
		Time:         nanoTime(timeNs),
		LastUpdateId: row.Sequence,
	}

	assign := func(src bookRow) {
		levels := levelsFrom(src.Data)
		if src.IsBid {
			book.Bids = levels
		} else {
			book.Asks = levels
		}
	}
	assign(row)
	if other != nil {
		assign(*other)
	}

	ev := marketdata.Event{
		Type:     evType,
		Exchange: r.exchangeOr(row.Exchange),
		Symbol:   book.Symbol,
		Key: marketdata.OrderKey{
			TimeNs: timeNs,
			Rank:   rank,
			Seq:    uint64(row.Sequence),
		},
		Book: book,
	}

	// firstUpdateId is Binance's "U", and an unbroken diff stream has
	// U == previous u + 1. Reporting PrevSeq as U-1 therefore lets BookState
	// prove contiguity, which it cannot do from the REST feed, where this field
	// is not exposed.
	if row.Metadata.FirstUpdateID > 0 {
		ev.PrevSeq = uint64(row.Metadata.FirstUpdateID - 1)
	}

	return ev
}

func (r *BookReader) symbolOr(fallback string) string {
	if r.symbol != "" {
		return r.symbol
	}
	return fallback
}

func (r *BookReader) exchangeOr(fallback string) types.ExchangeName {
	if r.exchange != "" {
		return r.exchange
	}
	return types.ExchangeName(fallback)
}

// Close releases the file.
func (r *BookReader) Close() error {
	if r.reader != nil {
		if err := r.reader.Close(); err != nil {
			r.file.Close()
			return err
		}
	}
	return r.file.Close()
}

// levelsFrom converts [[price, volume], ...] pairs.
//
// The values are float64 in the file, so converting them to fixedpoint cannot
// recover more precision than a float64 held. A malformed pair is skipped rather
// than failing the row: losing one level is better than losing the book.
func levelsFrom(data [][]float64) types.PriceVolumeSlice {
	if len(data) == 0 {
		return nil
	}

	out := make(types.PriceVolumeSlice, 0, len(data))
	for _, pair := range data {
		if len(pair) < 2 {
			continue
		}
		out = append(out, types.PriceVolume{
			Price:  fixedpoint.NewFromFloat(pair[0]),
			Volume: fixedpoint.NewFromFloat(pair[1]),
		})
	}

	return out
}

// InspectSchema returns a file's Parquet schema, for checking a new feature or a
// new venue before writing a decoder for it.
func InspectSchema(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", err
	}

	pf, err := parquet.OpenFile(f, st.Size())
	if err != nil {
		return "", fmt.Errorf("cloudsync: opening %s: %w", path, err)
	}

	return pf.Schema().String(), nil
}

// nanoTime converts a nanosecond epoch to a UTC time.
func nanoTime(ns int64) time.Time { return time.Unix(0, ns).UTC() }
