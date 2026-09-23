// Package mdtest provides shared test doubles and fixtures for the market data
// layer and its providers.
package mdtest

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/types"
)

// Events parses a compact text specification into events, so an ordering test
// reads as a table rather than as a wall of struct literals.
//
// One event per line. Blank lines and lines starting with // are ignored.
// The first field is always the event time in milliseconds; the second is the
// event type. Remaining fields are key=value pairs.
//
//	t=1000 trade price=100 qty=1 seq=5
//	t=1000 kline interval=1m o=1 h=2 l=0.5 c=1.5
//	t=1500 bookSnapshot bids=100,10 asks=105,20 seq=7
//	t=1600 bookUpdate bids=100,0 seq=8
//	t=1700 bookTicker bid=100,1 ask=101,2
//
// Recognized keys: sym (default BTCUSDT), ex (default binance), seq, ns
// (nanoseconds added to the millisecond time), flags.
func Events(t *testing.T, spec string) []marketdata.Event {
	t.Helper()

	var out []marketdata.Event
	for i, line := range strings.Split(spec, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		ev, err := parseEvent(line)
		if err != nil {
			t.Fatalf("mdtest: line %d (%q): %v", i+1, line, err)
		}
		out = append(out, ev)
	}

	return out
}

func parseEvent(line string) (marketdata.Event, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return marketdata.Event{}, errors.New("need at least a time and a type")
	}

	kv := map[string]string{}
	for _, f := range fields[2:] {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			return marketdata.Event{}, fmt.Errorf("field %q is not key=value", f)
		}
		kv[k] = v
	}

	tms, err := strconv.ParseInt(strings.TrimPrefix(fields[0], "t="), 10, 64)
	if err != nil {
		return marketdata.Event{}, fmt.Errorf("bad time %q: %w", fields[0], err)
	}

	timeNs := tms * 1e6
	if v, ok := kv["ns"]; ok {
		extra, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return marketdata.Event{}, fmt.Errorf("bad ns %q: %w", v, err)
		}
		timeNs += extra
	}

	var seq uint64
	if v, ok := kv["seq"]; ok {
		if seq, err = strconv.ParseUint(v, 10, 64); err != nil {
			return marketdata.Event{}, fmt.Errorf("bad seq %q: %w", v, err)
		}
	}

	symbol := "BTCUSDT"
	if v, ok := kv["sym"]; ok {
		symbol = v
	}

	exchange := types.ExchangeName("binance")
	if v, ok := kv["ex"]; ok {
		exchange = types.ExchangeName(v)
	}

	ev := marketdata.Event{
		Exchange: exchange,
		Symbol:   symbol,
		Key: marketdata.OrderKey{
			TimeNs: timeNs,
			Seq:    seq,
		},
	}
	eventTime := time.Unix(0, timeNs).UTC()

	switch fields[1] {
	case "trade":
		ev.Type = marketdata.EventTypeTrade
		ev.Key.Rank = marketdata.RankTrade
		ev.Trade = &types.Trade{
			ID:       seq,
			Exchange: exchange,
			Symbol:   symbol,
			Price:    num(kv, "price", 100),
			Quantity: num(kv, "qty", 1),
			Side:     types.SideTypeBuy,
			IsBuyer:  true,
			Time:     types.Time(eventTime),
		}

	case "kline":
		interval := types.Interval(get(kv, "interval", "1m"))
		ev.Type = marketdata.EventTypeKLine
		ev.Key.Rank = marketdata.KLineRank(interval)
		ev.KLine = &types.KLine{
			Exchange:  exchange,
			Symbol:    symbol,
			Interval:  interval,
			StartTime: types.Time(eventTime.Add(-interval.Duration())),
			EndTime:   types.Time(eventTime),
			Open:      num(kv, "o", 100),
			High:      num(kv, "h", 100),
			Low:       num(kv, "l", 100),
			Close:     num(kv, "c", 100),
			Volume:    num(kv, "v", 0),
			Closed:    true,
		}

	case "bookSnapshot", "bookUpdate":
		if fields[1] == "bookSnapshot" {
			ev.Type = marketdata.EventTypeBookSnapshot
			ev.Key.Rank = marketdata.RankBookSnapshot
		} else {
			ev.Type = marketdata.EventTypeBookUpdate
			ev.Key.Rank = marketdata.RankBookUpdate
		}

		bids, err := levels(kv["bids"])
		if err != nil {
			return marketdata.Event{}, fmt.Errorf("bids: %w", err)
		}
		asks, err := levels(kv["asks"])
		if err != nil {
			return marketdata.Event{}, fmt.Errorf("asks: %w", err)
		}

		ev.Book = &types.SliceOrderBook{
			Symbol:       symbol,
			Bids:         bids,
			Asks:         asks,
			Time:         eventTime,
			LastUpdateId: int64(seq),
		}

	case "bookTicker":
		ev.Type = marketdata.EventTypeBookTicker
		ev.Key.Rank = marketdata.RankBookTicker
		bid, err := level(get(kv, "bid", "100,1"))
		if err != nil {
			return marketdata.Event{}, fmt.Errorf("bid: %w", err)
		}
		ask, err := level(get(kv, "ask", "101,1"))
		if err != nil {
			return marketdata.Event{}, fmt.Errorf("ask: %w", err)
		}
		ev.BookTicker = &marketdata.BookTicker{
			BookTicker: types.BookTicker{
				Symbol:   symbol,
				Buy:      bid.Price,
				BuySize:  bid.Volume,
				Sell:     ask.Price,
				SellSize: ask.Volume,
			},
			UpdateID:        int64(seq),
			TransactionTime: types.Time(eventTime),
		}

	default:
		return marketdata.Event{}, fmt.Errorf("unknown event type %q", fields[1])
	}

	return ev, nil
}

func get(kv map[string]string, key, def string) string {
	if v, ok := kv[key]; ok {
		return v
	}
	return def
}

func num(kv map[string]string, key string, def float64) fixedpoint.Value {
	v, ok := kv[key]
	if !ok {
		return fixedpoint.NewFromFloat(def)
	}
	return fixedpoint.MustNewFromString(v)
}

// levels parses "100,10 101,5" into a PriceVolumeSlice.
func levels(s string) (types.PriceVolumeSlice, error) {
	if s == "" {
		return nil, nil
	}

	var out types.PriceVolumeSlice
	for _, part := range strings.Split(s, ";") {
		pv, err := level(part)
		if err != nil {
			return nil, err
		}
		out = append(out, pv)
	}
	return out, nil
}

func level(s string) (types.PriceVolume, error) {
	p, v, ok := strings.Cut(s, ",")
	if !ok {
		return types.PriceVolume{}, fmt.Errorf("level %q is not price,volume", s)
	}

	price, err := fixedpoint.NewFromString(p)
	if err != nil {
		return types.PriceVolume{}, err
	}
	volume, err := fixedpoint.NewFromString(v)
	if err != nil {
		return types.PriceVolume{}, err
	}

	return types.PriceVolume{Price: price, Volume: volume}, nil
}
