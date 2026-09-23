// Package grpcsource replays market data from a bbgo node over gRPC.
//
// It speaks the protocol in pkg/marketdata/pb, not the live MarketDataService in
// pkg/pb. This file is the only translation boundary between the wire types and
// marketdata.Event: the wire model uses strings for decimals and protobuf
// reflection state, while the in-process model uses fixedpoint and is cheap to
// copy, so keeping them separate stops a proto change rippling into every
// consumer.
package grpcsource

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/pb"
	"github.com/c9s/bbgo/pkg/types"
)

// eventTypes maps the wire enum to the in-process type.
var eventTypes = map[pb.EventType]marketdata.EventType{
	pb.EventType_EVENT_TYPE_KLINE:         marketdata.EventTypeKLine,
	pb.EventType_EVENT_TYPE_TRADE:         marketdata.EventTypeTrade,
	pb.EventType_EVENT_TYPE_BOOK_SNAPSHOT: marketdata.EventTypeBookSnapshot,
	pb.EventType_EVENT_TYPE_BOOK_UPDATE:   marketdata.EventTypeBookUpdate,
	pb.EventType_EVENT_TYPE_BOOK_TICKER:   marketdata.EventTypeBookTicker,
	pb.EventType_EVENT_TYPE_MARK_PRICE:    marketdata.EventTypeMarkPrice,
	pb.EventType_EVENT_TYPE_METRICS:       marketdata.EventTypeMetrics,
	pb.EventType_EVENT_TYPE_DEPTH_BAND:    marketdata.EventTypeDepthBand,
}

// wireEventTypes is the reverse mapping, for a server implementation.
var wireEventTypes = func() map[marketdata.EventType]pb.EventType {
	out := make(map[marketdata.EventType]pb.EventType, len(eventTypes))
	for wire, native := range eventTypes {
		out[native] = wire
	}
	return out
}()

// FromProto converts a wire event.
func FromProto(in *pb.Event) (marketdata.Event, error) {
	evType, ok := eventTypes[in.Type]
	if !ok {
		return marketdata.Event{}, fmt.Errorf("grpcsource: unsupported event type %s", in.Type)
	}

	if in.EventTimeNs == 0 {
		// This protocol has no legacy senders to be lenient towards: a zero
		// timestamp would sort before every other event in a merge, so it is an
		// error rather than something to guess around.
		return marketdata.Event{}, fmt.Errorf(
			"grpcsource: %s event for %s has no event_time_ns", in.Type, in.Symbol)
	}

	out := marketdata.Event{
		Type:     evType,
		Flags:    marketdata.EventFlag(in.Flags),
		Exchange: types.ExchangeName(in.Exchange),
		Symbol:   in.Symbol,
		PrevSeq:  in.PrevSequence,
		Key: marketdata.OrderKey{
			TimeNs: in.EventTimeNs,
			Rank:   marketdata.RankOf(evType),
			Seq:    in.Sequence,
		},
	}

	switch payload := in.Payload.(type) {
	case *pb.Event_Book:
		book, err := bookFromProto(payload.Book, in)
		if err != nil {
			return out, err
		}
		out.Book = book

	case *pb.Event_Trade:
		trade, err := tradeFromProto(payload.Trade, in)
		if err != nil {
			return out, err
		}
		out.Trade = trade

	case *pb.Event_Kline:
		kline, err := klineFromProto(payload.Kline, in)
		if err != nil {
			return out, err
		}
		out.KLine = kline
		out.Key.Rank = marketdata.KLineRank(kline.Interval)

	case *pb.Event_BookTicker:
		ticker, err := bookTickerFromProto(payload.BookTicker, in)
		if err != nil {
			return out, err
		}
		out.BookTicker = ticker

	case *pb.Event_MarkPrice:
		mark, err := markPriceFromProto(payload.MarkPrice, in)
		if err != nil {
			return out, err
		}
		out.MarkPrice = mark

	case *pb.Event_Metrics:
		metrics, err := metricsFromProto(payload.Metrics, in)
		if err != nil {
			return out, err
		}
		out.Metrics = metrics

	case *pb.Event_DepthBand:
		band, err := depthBandFromProto(payload.DepthBand, in)
		if err != nil {
			return out, err
		}
		out.DepthBand = band

	case nil:
		return out, fmt.Errorf("grpcsource: %s event for %s has no payload", in.Type, in.Symbol)

	default:
		return out, fmt.Errorf("grpcsource: unhandled payload %T", payload)
	}

	return out, nil
}

// ToProto converts an event for a server to send.
func ToProto(in *marketdata.Event) (*pb.Event, error) {
	wireType, ok := wireEventTypes[in.Type]
	if !ok {
		return nil, fmt.Errorf("grpcsource: cannot send event type %s", in.Type)
	}

	out := &pb.Event{
		Type:         wireType,
		Exchange:     string(in.Exchange),
		Symbol:       in.Symbol,
		EventTimeNs:  in.Key.TimeNs,
		Sequence:     in.Key.Seq,
		PrevSequence: in.PrevSeq,
		Flags:        uint32(in.Flags),
	}

	switch {
	case in.Book != nil:
		out.Payload = &pb.Event_Book{Book: &pb.Book{
			Asks:         levelsToProto(in.Book.Asks),
			Bids:         levelsToProto(in.Book.Bids),
			LastUpdateId: in.Book.LastUpdateId,
		}}

	case in.Trade != nil:
		out.Payload = &pb.Event_Trade{Trade: &pb.Trade{
			Id:            fmt.Sprintf("%d", in.Trade.ID),
			Price:         in.Trade.Price.String(),
			Quantity:      in.Trade.Quantity.String(),
			QuoteQuantity: in.Trade.QuoteQuantity.String(),
			IsBuyer:       in.Trade.IsBuyer,
			IsFutures:     in.Trade.IsFutures,
		}}

	case in.KLine != nil:
		out.Payload = &pb.Event_Kline{Kline: &pb.KLine{
			Interval:       string(in.KLine.Interval),
			StartTimeNs:    in.KLine.StartTime.Time().UnixNano(),
			Open:           in.KLine.Open.String(),
			High:           in.KLine.High.String(),
			Low:            in.KLine.Low.String(),
			Close:          in.KLine.Close.String(),
			Volume:         in.KLine.Volume.String(),
			QuoteVolume:    in.KLine.QuoteVolume.String(),
			NumberOfTrades: in.KLine.NumberOfTrades,
			Closed:         in.KLine.Closed,
		}}

	case in.BookTicker != nil:
		out.Payload = &pb.Event_BookTicker{BookTicker: &pb.BookTicker{
			BestBidPrice:      in.BookTicker.Buy.String(),
			BestBidQty:        in.BookTicker.BuySize.String(),
			BestAskPrice:      in.BookTicker.Sell.String(),
			BestAskQty:        in.BookTicker.SellSize.String(),
			UpdateId:          in.BookTicker.UpdateID,
			TransactionTimeNs: in.BookTicker.TransactionTime.Time().UnixNano(),
			PublishTimeNs:     in.BookTicker.EventTime.Time().UnixNano(),
		}}

	default:
		return nil, fmt.Errorf("grpcsource: %s event has no payload to send", in.Type)
	}

	return out, nil
}

func bookFromProto(in *pb.Book, envelope *pb.Event) (*types.SliceOrderBook, error) {
	asks, err := levelsFromProto(in.Asks)
	if err != nil {
		return nil, fmt.Errorf("grpcsource: asks: %w", err)
	}
	bids, err := levelsFromProto(in.Bids)
	if err != nil {
		return nil, fmt.Errorf("grpcsource: bids: %w", err)
	}

	lastUpdateID := in.LastUpdateId
	if lastUpdateID == 0 {
		lastUpdateID = int64(envelope.Sequence)
	}

	return &types.SliceOrderBook{
		Symbol:       envelope.Symbol,
		Asks:         asks,
		Bids:         bids,
		Time:         nanoTime(envelope.EventTimeNs),
		LastUpdateId: lastUpdateID,
	}, nil
}

func tradeFromProto(in *pb.Trade, envelope *pb.Event) (*types.Trade, error) {
	price, err := fixedpoint.NewFromString(in.Price)
	if err != nil {
		return nil, fmt.Errorf("grpcsource: trade price %q: %w", in.Price, err)
	}
	quantity, err := fixedpoint.NewFromString(in.Quantity)
	if err != nil {
		return nil, fmt.Errorf("grpcsource: trade quantity %q: %w", in.Quantity, err)
	}

	quoteQuantity := price.Mul(quantity)
	if in.QuoteQuantity != "" {
		if v, err := fixedpoint.NewFromString(in.QuoteQuantity); err == nil {
			quoteQuantity = v
		}
	}

	side := types.SideTypeSell
	if in.IsBuyer {
		side = types.SideTypeBuy
	}

	return &types.Trade{
		ID:            envelope.Sequence,
		Exchange:      types.ExchangeName(envelope.Exchange),
		Symbol:        envelope.Symbol,
		Price:         price,
		Quantity:      quantity,
		QuoteQuantity: quoteQuantity,
		Side:          side,
		IsBuyer:       in.IsBuyer,
		IsFutures:     in.IsFutures,
		Time:          types.Time(nanoTime(envelope.EventTimeNs)),
	}, nil
}

func klineFromProto(in *pb.KLine, envelope *pb.Event) (*types.KLine, error) {
	kline := &types.KLine{
		Exchange:       types.ExchangeName(envelope.Exchange),
		Symbol:         envelope.Symbol,
		Interval:       types.Interval(in.Interval),
		StartTime:      types.Time(nanoTime(in.StartTimeNs)),
		EndTime:        types.Time(nanoTime(envelope.EventTimeNs)),
		NumberOfTrades: in.NumberOfTrades,
		Closed:         in.Closed,
	}

	for _, f := range []struct {
		name string
		in   string
		out  *fixedpoint.Value
	}{
		{"open", in.Open, &kline.Open},
		{"high", in.High, &kline.High},
		{"low", in.Low, &kline.Low},
		{"close", in.Close, &kline.Close},
		{"volume", in.Volume, &kline.Volume},
		{"quoteVolume", in.QuoteVolume, &kline.QuoteVolume},
	} {
		if f.in == "" {
			continue
		}
		v, err := fixedpoint.NewFromString(f.in)
		if err != nil {
			return nil, fmt.Errorf("grpcsource: kline %s %q: %w", f.name, f.in, err)
		}
		*f.out = v
	}

	return kline, nil
}

func bookTickerFromProto(in *pb.BookTicker, envelope *pb.Event) (*marketdata.BookTicker, error) {
	ticker := &marketdata.BookTicker{
		BookTicker:      types.BookTicker{Symbol: envelope.Symbol},
		UpdateID:        in.UpdateId,
		TransactionTime: types.Time(nanoTime(in.TransactionTimeNs)),
		EventTime:       types.Time(nanoTime(in.PublishTimeNs)),
	}

	for _, f := range []struct {
		name string
		in   string
		out  *fixedpoint.Value
	}{
		{"bestBidPrice", in.BestBidPrice, &ticker.Buy},
		{"bestBidQty", in.BestBidQty, &ticker.BuySize},
		{"bestAskPrice", in.BestAskPrice, &ticker.Sell},
		{"bestAskQty", in.BestAskQty, &ticker.SellSize},
	} {
		if f.in == "" {
			continue
		}
		v, err := fixedpoint.NewFromString(f.in)
		if err != nil {
			return nil, fmt.Errorf("grpcsource: bookTicker %s %q: %w", f.name, f.in, err)
		}
		*f.out = v
	}

	return ticker, nil
}

func markPriceFromProto(in *pb.MarkPrice, envelope *pb.Event) (*marketdata.MarkPrice, error) {
	mark := &marketdata.MarkPrice{
		Symbol:          envelope.Symbol,
		NextFundingTime: types.Time(nanoTime(in.NextFundingTimeNs)),
		Time:            types.Time(nanoTime(envelope.EventTimeNs)),
	}

	for _, f := range []struct {
		name string
		in   string
		out  *fixedpoint.Value
	}{
		{"markPrice", in.MarkPrice, &mark.MarkPrice},
		{"indexPrice", in.IndexPrice, &mark.IndexPrice},
		{"estimatedSettlePrice", in.EstimatedSettlePrice, &mark.EstimatedSettlePrice},
		{"lastFundingRate", in.LastFundingRate, &mark.LastFundingRate},
	} {
		if f.in == "" {
			continue
		}
		v, err := fixedpoint.NewFromString(f.in)
		if err != nil {
			return nil, fmt.Errorf("grpcsource: markPrice %s %q: %w", f.name, f.in, err)
		}
		*f.out = v
	}

	return mark, nil
}

func metricsFromProto(in *pb.Metrics, envelope *pb.Event) (*marketdata.Metrics, error) {
	values := make(map[string]fixedpoint.Value, len(in.Values))
	for key, raw := range in.Values {
		v, err := fixedpoint.NewFromString(raw)
		if err != nil {
			return nil, fmt.Errorf("grpcsource: metric %s %q: %w", key, raw, err)
		}
		values[key] = v
	}

	return &marketdata.Metrics{
		Symbol: envelope.Symbol,
		Time:   types.Time(nanoTime(envelope.EventTimeNs)),
		Values: values,
	}, nil
}

func depthBandFromProto(in *pb.DepthBand, envelope *pb.Event) (*marketdata.DepthBand, error) {
	band := &marketdata.DepthBand{
		Symbol: envelope.Symbol,
		Time:   types.Time(nanoTime(envelope.EventTimeNs)),
	}

	for _, f := range []struct {
		name string
		in   string
		out  *fixedpoint.Value
	}{
		{"percentage", in.Percentage, &band.Percentage},
		{"depth", in.Depth, &band.Depth},
		{"notional", in.Notional, &band.Notional},
	} {
		if f.in == "" {
			continue
		}
		v, err := fixedpoint.NewFromString(f.in)
		if err != nil {
			return nil, fmt.Errorf("grpcsource: depthBand %s %q: %w", f.name, f.in, err)
		}
		*f.out = v
	}

	return band, nil
}

func levelsFromProto(in []*pb.PriceVolume) (types.PriceVolumeSlice, error) {
	if len(in) == 0 {
		return nil, nil
	}

	out := make(types.PriceVolumeSlice, len(in))
	for i, pv := range in {
		price, err := fixedpoint.NewFromString(pv.Price)
		if err != nil {
			return nil, fmt.Errorf("price %q: %w", pv.Price, err)
		}
		volume, err := fixedpoint.NewFromString(pv.Volume)
		if err != nil {
			return nil, fmt.Errorf("volume %q: %w", pv.Volume, err)
		}
		out[i] = types.PriceVolume{Price: price, Volume: volume}
	}
	return out, nil
}

func levelsToProto(in types.PriceVolumeSlice) []*pb.PriceVolume {
	if len(in) == 0 {
		return nil
	}

	out := make([]*pb.PriceVolume, len(in))
	for i, pv := range in {
		out[i] = &pb.PriceVolume{Price: pv.Price.String(), Volume: pv.Volume.String()}
	}
	return out
}
