package amberdata

import (
	"time"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/sources/amberdata/amberdataapi"
	"github.com/c9s/bbgo/pkg/types"
)

func nanoTime(ns int64) time.Time { return time.Unix(0, ns).UTC() }

// tradeToEvent converts an API trade.
//
// isBuySide already reports the aggressor side, so unlike Binance's
// is_buyer_maker it needs no inversion. IsMaker stays false: it means "my order
// was the maker" and has no meaning for a public trade.
func tradeToEvent(
	in amberdataapi.Trade, exchange types.ExchangeName, symbol string,
) marketdata.Event {
	timeNs := in.EventTimeNano()

	side := types.SideTypeSell
	if in.IsBuySide {
		side = types.SideTypeBuy
	}

	trade := &types.Trade{
		ID:        in.Sequence.Value,
		Exchange:  exchange,
		Symbol:    symbol,
		Price:     in.Price.Value,
		Quantity:  in.Volume.Value,
		Side:      side,
		IsBuyer:   in.IsBuySide,
		IsMaker:   false,
		IsFutures: true,
		Time:      types.Time(nanoTime(timeNs)),
	}

	if in.QuoteVolume.Valid {
		trade.QuoteQuantity = in.QuoteVolume.Value
	} else {
		// quoteVolume is null on several venues including Binance, so it is
		// derived rather than left at zero.
		trade.QuoteQuantity = in.Price.Value.Mul(in.Volume.Value)
	}

	// tradeId is a string and is not always numeric, so it is only used as an
	// ordering tiebreaker when it parses.
	seq := in.Sequence.Value
	if !in.Sequence.Valid {
		if parsed, ok := parseUint(in.TradeID); ok {
			seq = parsed
			trade.ID = parsed
		}
	}

	return marketdata.Event{
		Type:     marketdata.EventTypeTrade,
		Exchange: exchange,
		Symbol:   symbol,
		Key: marketdata.OrderKey{
			TimeNs: timeNs,
			Rank:   marketdata.RankTrade,
			Seq:    seq,
		},
		Trade: trade,
	}
}

// bookToEvent converts an API book record.
//
// The snapshots and events endpoints return the same shape and there is no flag
// distinguishing them, so which one it is comes from the caller. A level with
// volume zero in an events record means removal, which is exactly what
// types.SliceOrderBook.Update already implements, so the levels pass through
// untouched.
//
// Event.PrevSeq is deliberately left zero. The REST response carries only
// `sequence`, with no equivalent of Binance's first-update id, so there is
// nothing to say which event this one follows and claiming otherwise would be a
// guess; a consumer sees these as monotonic-but-unverified. The CloudSync flat
// files do carry it as metadata.firstUpdateId, which is why the cloudsync
// package can populate PrevSeq and this cannot.
func bookToEvent(
	in amberdataapi.Book, exchange types.ExchangeName, symbol string,
	snapshot bool, truncated bool,
) marketdata.Event {
	timeNs := in.EventTimeNano()

	evType := marketdata.EventTypeBookUpdate
	rank := marketdata.RankBookUpdate
	if snapshot {
		evType = marketdata.EventTypeBookSnapshot
		rank = marketdata.RankBookSnapshot
	}

	var flags marketdata.EventFlag
	if truncated {
		flags |= marketdata.FlagPartialDepth
	}

	return marketdata.Event{
		Type:     evType,
		Flags:    flags,
		Exchange: exchange,
		Symbol:   symbol,
		Key: marketdata.OrderKey{
			TimeNs: timeNs,
			Rank:   rank,
			Seq:    in.Sequence.Value,
		},
		Book: &types.SliceOrderBook{
			Symbol:       symbol,
			Asks:         levelsToSlice(in.Ask),
			Bids:         levelsToSlice(in.Bid),
			Time:         nanoTime(timeNs),
			LastUpdateId: int64(in.Sequence.Value),
		},
	}
}

// ohlcvToEvent converts an API candle.
//
// The API reports the bucket's start time, while a kline event happens at its
// close, so the end is derived from the interval and used as the ordering key.
func ohlcvToEvent(
	in amberdataapi.OHLCV, exchange types.ExchangeName, symbol string, interval types.Interval,
) marketdata.Event {
	startNs := in.ExchangeTimestamp.UnixNano()
	endNs := startNs + interval.Duration().Nanoseconds() - int64(time.Millisecond)

	return marketdata.Event{
		Type:     marketdata.EventTypeKLine,
		Exchange: exchange,
		Symbol:   symbol,
		Key: marketdata.OrderKey{
			TimeNs: endNs,
			Rank:   marketdata.KLineRank(interval),
		},
		KLine: &types.KLine{
			Exchange:  exchange,
			Symbol:    symbol,
			Interval:  interval,
			StartTime: types.Time(nanoTime(startNs)),
			EndTime:   types.Time(nanoTime(endNs)),
			Open:      in.Open.Value,
			High:      in.High.Value,
			Low:       in.Low.Value,
			Close:     in.Close.Value,
			Volume:    in.Volume.Value,
			Closed:    true,
		},
	}
}

func levelsToSlice(in []amberdataapi.PriceLevel) types.PriceVolumeSlice {
	if len(in) == 0 {
		return nil
	}

	out := make(types.PriceVolumeSlice, len(in))
	for i, level := range in {
		out[i] = types.PriceVolume{Price: level.Price.Value, Volume: level.Volume.Value}
	}
	return out
}

func parseUint(s string) (uint64, bool) {
	var v uint64
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		v = v*10 + uint64(r-'0')
	}
	return v, true
}
