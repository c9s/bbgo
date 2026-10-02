package binancecsv

import (
	"fmt"
	"strconv"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/types"
)

// aggTrade column names, as published in the futures archives. The spot
// archives carry the same fields in the same order with no header row, plus a
// trailing is_best_match column.
var aggTradeColumns = []string{
	"agg_trade_id", "price", "quantity",
	"first_trade_id", "last_trade_id", "transact_time", "is_buyer_maker",
}

type aggTradeDecoder struct{ cfg Config }

func newAggTradeDecoder(cfg Config) RecordDecoder { return &aggTradeDecoder{cfg: cfg} }

func (d *aggTradeDecoder) Columns() []string { return aggTradeColumns }

func (d *aggTradeDecoder) Decode(
	dst []marketdata.Event, record []string, meta RecordMeta,
) ([]marketdata.Event, error) {
	if len(record) < 7 {
		return dst, fmt.Errorf("binancecsv: aggTrades record has %d fields, want at least 7", len(record))
	}

	id, err := strconv.ParseUint(record[meta.column("agg_trade_id", 0)], 10, 64)
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad agg_trade_id: %w", err)
	}

	trade, err := decodeTradeFields(
		record[meta.column("price", 1)],
		record[meta.column("quantity", 2)],
		record[meta.column("transact_time", 5)],
		record[meta.column("is_buyer_maker", 6)],
		meta,
	)
	if err != nil {
		return dst, err
	}
	trade.trade.ID = id

	ev := marketdata.Event{
		Type:     marketdata.EventTypeTrade,
		Flags:    marketdata.FlagAggregated,
		Exchange: meta.Exchange,
		Symbol:   meta.Symbol,
		Key: marketdata.OrderKey{
			TimeNs: trade.timeNs,
			Rank:   marketdata.RankTrade,
			Seq:    id,
		},
		Trade: &trade.trade,
	}

	return append(dst, ev), nil
}

// trades column names, futures layout. Spot publishes the same fields with two
// extra trailing booleans and no header.
var tradeColumns = []string{"id", "price", "qty", "quote_qty", "time", "is_buyer_maker"}

type tradeDecoder struct{ cfg Config }

func newTradeDecoder(cfg Config) RecordDecoder { return &tradeDecoder{cfg: cfg} }

func (d *tradeDecoder) Columns() []string { return tradeColumns }

func (d *tradeDecoder) Decode(
	dst []marketdata.Event, record []string, meta RecordMeta,
) ([]marketdata.Event, error) {
	if len(record) < 6 {
		return dst, fmt.Errorf("binancecsv: trades record has %d fields, want at least 6", len(record))
	}

	id, err := strconv.ParseUint(record[meta.column("id", 0)], 10, 64)
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad trade id: %w", err)
	}

	trade, err := decodeTradeFields(
		record[meta.column("price", 1)],
		record[meta.column("qty", 2)],
		record[meta.column("time", 4)],
		record[meta.column("is_buyer_maker", 5)],
		meta,
	)
	if err != nil {
		return dst, err
	}
	trade.trade.ID = id

	// The trades dataset publishes quote_qty directly; prefer it over the
	// product, which loses precision on assets with many decimals.
	if i := meta.column("quote_qty", 3); i < len(record) {
		if qq, err := fixedpoint.NewFromString(record[i]); err == nil {
			trade.trade.QuoteQuantity = qq
		}
	}

	ev := marketdata.Event{
		Type:     marketdata.EventTypeTrade,
		Exchange: meta.Exchange,
		Symbol:   meta.Symbol,
		Key: marketdata.OrderKey{
			TimeNs: trade.timeNs,
			Rank:   marketdata.RankTrade,
			Seq:    id,
		},
		Trade: &trade.trade,
	}

	return append(dst, ev), nil
}

type decodedTrade struct {
	trade  types.Trade
	timeNs int64
}

// decodeTradeFields builds the shared part of a public trade.
//
// The is_buyer_maker mapping is the subtle one, and the previous implementation
// got it wrong. is_buyer_maker reports which side was resting: when it is true
// the buyer was the maker, so the incoming aggressor was a seller.
//
// types.Trade.IsMaker means "my order was the maker" — it is a private-trade
// field, meaningful only for the account's own fills. The old code set it from
// is_buyer_maker, which made every public sell trade come out as IsMaker=true
// and would silently corrupt any fee model keyed off it. A public trade has no
// "my" side, so IsMaker is false here.
func decodeTradeFields(
	priceStr, qtyStr, timeStr, isBuyerMakerStr string, meta RecordMeta,
) (decodedTrade, error) {
	price, err := fixedpoint.NewFromString(priceStr)
	if err != nil {
		return decodedTrade{}, fmt.Errorf("binancecsv: bad price %q: %w", priceStr, err)
	}

	quantity, err := fixedpoint.NewFromString(qtyStr)
	if err != nil {
		return decodedTrade{}, fmt.Errorf("binancecsv: bad quantity %q: %w", qtyStr, err)
	}

	timeNs, err := parseEventTimeNano(timeStr)
	if err != nil {
		return decodedTrade{}, err
	}

	// ParseBool accepts both the futures "true"/"false" and the Python-style
	// "True"/"False" the spot archives use.
	isBuyerMaker, err := strconv.ParseBool(isBuyerMakerStr)
	if err != nil {
		return decodedTrade{}, fmt.Errorf("binancecsv: bad is_buyer_maker %q: %w", isBuyerMakerStr, err)
	}

	side := types.SideTypeBuy
	if isBuyerMaker {
		side = types.SideTypeSell
	}

	return decodedTrade{
		timeNs: timeNs,
		trade: types.Trade{
			Exchange:      meta.Exchange,
			Symbol:        meta.Symbol,
			Price:         price,
			Quantity:      quantity,
			QuoteQuantity: price.Mul(quantity),
			Side:          side,
			IsBuyer:       !isBuyerMaker,
			IsMaker:       false,
			IsFutures:     meta.Market.IsFutures(),
			Time:          types.Time(nanoTime(timeNs)),
		},
	}, nil
}
