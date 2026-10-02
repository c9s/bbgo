package binancecsv

import (
	"fmt"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/types"
)

// kline column names, shared by klines, markPriceKlines, indexPriceKlines and
// premiumIndexKlines.
var klineColumns = []string{
	"open_time", "open", "high", "low", "close", "volume",
	"close_time", "quote_volume", "count", "taker_buy_volume", "taker_buy_quote_volume", "ignore",
}

type klineDecoder struct{ cfg Config }

func newKLineDecoder(cfg Config) RecordDecoder { return &klineDecoder{cfg: cfg} }

func (d *klineDecoder) Columns() []string { return klineColumns }

func (d *klineDecoder) Decode(
	dst []marketdata.Event, record []string, meta RecordMeta,
) ([]marketdata.Event, error) {
	if len(record) < 6 {
		return dst, fmt.Errorf("binancecsv: kline record has %d fields, want at least 6", len(record))
	}

	openNs, err := parseEventTimeNano(record[meta.column("open_time", 0)])
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad open_time: %w", err)
	}

	kline := types.KLine{
		Exchange:  meta.Exchange,
		Symbol:    meta.Symbol,
		Interval:  meta.Interval,
		StartTime: types.Time(nanoTime(openNs)),
		Closed:    true,
	}

	for _, f := range []struct {
		name  string
		pos   int
		dst   *fixedpoint.Value
		mustP bool
	}{
		{"open", 1, &kline.Open, true},
		{"high", 2, &kline.High, true},
		{"low", 3, &kline.Low, true},
		{"close", 4, &kline.Close, true},
		{"volume", 5, &kline.Volume, false},
		{"quote_volume", 7, &kline.QuoteVolume, false},
		{"taker_buy_volume", 9, &kline.TakerBuyBaseAssetVolume, false},
		{"taker_buy_quote_volume", 10, &kline.TakerBuyQuoteAssetVolume, false},
	} {
		i := meta.column(f.name, f.pos)
		if i >= len(record) {
			if f.mustP {
				return dst, fmt.Errorf("binancecsv: kline record is missing %s", f.name)
			}
			continue
		}

		v, err := fixedpoint.NewFromString(record[i])
		if err != nil {
			if f.mustP {
				return dst, fmt.Errorf("binancecsv: bad %s %q: %w", f.name, record[i], err)
			}
			continue
		}
		*f.dst = v
	}

	if i := meta.column("count", 8); i < len(record) {
		if n, err := strconv.ParseUint(record[i], 10, 64); err == nil {
			kline.NumberOfTrades = n
		}
	}

	// Prefer the published close_time: it is the venue's own value, and it
	// already follows the Binance convention of ending one millisecond before
	// the next candle opens.
	endNs := openNs + meta.Interval.Duration().Nanoseconds() - int64(time.Millisecond)
	if i := meta.column("close_time", 6); i < len(record) {
		if v, err := parseEventTimeNano(record[i]); err == nil {
			endNs = v
		}
	}
	kline.EndTime = types.Time(nanoTime(endNs))

	// A kline event happens at its close: that is the instant the information
	// becomes observable, and it is what makes the ordering against trades and
	// book updates at the same timestamp correct.
	ev := marketdata.Event{
		Type:     marketdata.EventTypeKLine,
		Exchange: meta.Exchange,
		Symbol:   meta.Symbol,
		Key: marketdata.OrderKey{
			TimeNs: endNs,
			Rank:   marketdata.KLineRank(meta.Interval),
		},
		KLine: &kline,
	}

	return append(dst, ev), nil
}

func nanoTime(ns int64) time.Time { return time.Unix(0, ns).UTC() }
