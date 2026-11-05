package replay

import (
	"encoding/json"
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/types"
)

// EncodeEvent converts an event into a record. It returns false for event types
// the format does not carry, so a recorder can skip them silently.
func EncodeEvent(ev *marketdata.Event) (Record, bool) {
	r := Record{
		T:  ev.Key.TimeNs,
		Ty: ev.Type,
		Ex: ev.Exchange,
		S:  ev.Symbol,
		Q:  ev.Key.Seq,
		Pu: int64(ev.PrevSeq),
		F:  ev.Flags,
	}

	var payload any

	switch ev.Type {
	case marketdata.EventTypeBookSnapshot, marketdata.EventTypeBookUpdate:
		if ev.Book == nil {
			return Record{}, false
		}
		payload = bookPayload{
			Bids: levelsToPairs(ev.Book.Bids),
			Asks: levelsToPairs(ev.Book.Asks),
		}

	case marketdata.EventTypeTrade:
		if ev.Trade == nil {
			return Record{}, false
		}
		payload = tradePayload{
			Price:    ev.Trade.Price.String(),
			Quantity: ev.Trade.Quantity.String(),
			Buy:      ev.Trade.Side == types.SideTypeBuy,
			ID:       ev.Trade.ID,
		}

	case marketdata.EventTypeKLine:
		if ev.KLine == nil {
			return Record{}, false
		}
		payload = klinePayload{
			Interval:    ev.KLine.Interval,
			StartTime:   ev.KLine.StartTime.Time().UnixNano(),
			Open:        ev.KLine.Open.String(),
			High:        ev.KLine.High.String(),
			Low:         ev.KLine.Low.String(),
			Close:       ev.KLine.Close.String(),
			Volume:      ev.KLine.Volume.String(),
			QuoteVolume: ev.KLine.QuoteVolume.String(),
		}

	case marketdata.EventTypeBookTicker:
		if ev.BookTicker == nil {
			return Record{}, false
		}
		payload = bookTickerPayload{
			BidPrice: ev.BookTicker.Buy.String(),
			BidSize:  ev.BookTicker.BuySize.String(),
			AskPrice: ev.BookTicker.Sell.String(),
			AskSize:  ev.BookTicker.SellSize.String(),
			EventT:   ev.BookTicker.EventTime.Time().UnixNano(),
		}

	default:
		return Record{}, false
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		// Every payload above is a plain struct of strings and ints, so this is
		// unreachable; dropping the record is still better than panicking in a
		// recorder that is capturing a live feed.
		return Record{}, false
	}
	r.P = encoded

	return r, true
}

// DecodeRecord converts a record back into an event, applying the header's
// exchange and symbol when the record omits them.
func DecodeRecord(r Record, header Header) (marketdata.Event, error) {
	ev := marketdata.Event{
		Type:     r.Ty,
		Flags:    r.F,
		Exchange: r.Ex,
		Symbol:   r.S,
		PrevSeq:  uint64(r.Pu),
		Key: marketdata.OrderKey{
			TimeNs: r.T,
			Rank:   marketdata.RankOf(r.Ty),
			Seq:    r.Q,
		},
	}

	if ev.Exchange == "" {
		ev.Exchange = header.Exchange
	}
	if ev.Symbol == "" && len(header.Symbols) == 1 {
		ev.Symbol = header.Symbols[0]
	}

	switch r.Ty {
	case marketdata.EventTypeBookSnapshot, marketdata.EventTypeBookUpdate:
		var p bookPayload
		if err := json.Unmarshal(r.P, &p); err != nil {
			return ev, recordError(r, err)
		}

		bids, err := pairsToLevels(p.Bids)
		if err != nil {
			return ev, recordError(r, err)
		}
		asks, err := pairsToLevels(p.Asks)
		if err != nil {
			return ev, recordError(r, err)
		}

		ev.Book = &types.SliceOrderBook{
			Symbol:       ev.Symbol,
			Bids:         bids,
			Asks:         asks,
			Time:         nanoTime(r.T),
			LastUpdateId: int64(r.Q),
		}

	case marketdata.EventTypeTrade:
		var p tradePayload
		if err := json.Unmarshal(r.P, &p); err != nil {
			return ev, recordError(r, err)
		}

		price, err := fixedpoint.NewFromString(p.Price)
		if err != nil {
			return ev, recordError(r, err)
		}
		quantity, err := fixedpoint.NewFromString(p.Quantity)
		if err != nil {
			return ev, recordError(r, err)
		}

		side := types.SideTypeSell
		if p.Buy {
			side = types.SideTypeBuy
		}

		ev.Trade = &types.Trade{
			ID:            p.ID,
			Exchange:      ev.Exchange,
			Symbol:        ev.Symbol,
			Price:         price,
			Quantity:      quantity,
			QuoteQuantity: price.Mul(quantity),
			Side:          side,
			IsBuyer:       p.Buy,
			Time:          types.Time(nanoTime(r.T)),
		}

	case marketdata.EventTypeKLine:
		var p klinePayload
		if err := json.Unmarshal(r.P, &p); err != nil {
			return ev, recordError(r, err)
		}

		kline := types.KLine{
			Exchange:  ev.Exchange,
			Symbol:    ev.Symbol,
			Interval:  p.Interval,
			StartTime: types.Time(nanoTime(p.StartTime)),
			EndTime:   types.Time(nanoTime(r.T)),
			Closed:    true,
		}
		for _, f := range []struct {
			in  string
			out *fixedpoint.Value
		}{
			{p.Open, &kline.Open},
			{p.High, &kline.High},
			{p.Low, &kline.Low},
			{p.Close, &kline.Close},
			{p.Volume, &kline.Volume},
			{p.QuoteVolume, &kline.QuoteVolume},
		} {
			if f.in == "" {
				continue
			}
			v, err := fixedpoint.NewFromString(f.in)
			if err != nil {
				return ev, recordError(r, err)
			}
			*f.out = v
		}

		ev.KLine = &kline
		ev.Key.Rank = marketdata.KLineRank(p.Interval)

	case marketdata.EventTypeBookTicker:
		var p bookTickerPayload
		if err := json.Unmarshal(r.P, &p); err != nil {
			return ev, recordError(r, err)
		}

		ticker := &marketdata.BookTicker{
			BookTicker:      types.BookTicker{Symbol: ev.Symbol},
			UpdateID:        int64(r.Q),
			TransactionTime: types.Time(nanoTime(r.T)),
			EventTime:       types.Time(nanoTime(p.EventT)),
		}
		for _, f := range []struct {
			in  string
			out *fixedpoint.Value
		}{
			{p.BidPrice, &ticker.Buy},
			{p.BidSize, &ticker.BuySize},
			{p.AskPrice, &ticker.Sell},
			{p.AskSize, &ticker.SellSize},
		} {
			if f.in == "" {
				continue
			}
			v, err := fixedpoint.NewFromString(f.in)
			if err != nil {
				return ev, recordError(r, err)
			}
			*f.out = v
		}

		ev.BookTicker = ticker

	default:
		return ev, fmt.Errorf("replay: unsupported record type %s", r.Ty)
	}

	return ev, nil
}
