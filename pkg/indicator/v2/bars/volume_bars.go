package bars

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfVolumeBars = 5_000

// VolumeBarStream aggregates trades into fixed-volume KLines ("volume bars"): once the
// accumulated trade volume of the current bar reaches barVolume, the bar closes and a new
// one starts. A trade that crosses the threshold is split across the boundary: the portion
// that fills the current bar closes it exactly at barVolume, and the remaining portion opens
// the next bar (counted as 1 trade in that bar's NumberOfTrades). barVolume must be positive.
//
//go:generate callbackgen -type VolumeBarStream
type VolumeBarStream struct {
	updateCallbacks []func(k types.KLine)

	barVolume fixedpoint.Value
	klines    []types.KLine

	current       types.KLine
	currentVolume fixedpoint.Value
}

func (s *VolumeBarStream) Length() int {
	return len(s.klines)
}

func (s *VolumeBarStream) Last(i int) *types.KLine {
	l := len(s.klines)
	if i < 0 || l-1-i < 0 {
		return nil
	}

	return &s.klines[l-1-i]
}

func (s *VolumeBarStream) Tail(n int) types.KLineWindow {
	l := len(s.klines)
	if n >= l {
		return s.klines
	}

	return s.klines[l-n : l]
}

// AddSubscriber adds the subscriber function and pushes historical volume bars to the subscriber
func (s *VolumeBarStream) AddSubscriber(f func(k types.KLine)) {
	s.OnUpdate(f)

	if len(s.klines) == 0 {
		return
	}

	// push historical volume bars to the subscriber
	for _, k := range s.klines {
		f(k)
	}
}

// addTrade folds a trade into the currently accumulating bar, splitting it across a bar
// boundary (and possibly several, if the trade's volume exceeds barVolume) as needed.
func (s *VolumeBarStream) addTrade(trade types.Trade) {
	remainingQty := trade.Quantity

	for {
		capacity := s.barVolume.Sub(s.currentVolume)
		fillQty := fixedpoint.Min(remainingQty, capacity)

		s.foldFill(trade, fillQty)
		s.currentVolume = s.currentVolume.Add(fillQty)
		remainingQty = remainingQty.Sub(fillQty)

		if s.currentVolume.Compare(s.barVolume) < 0 {
			return
		}

		s.closeCurrentBar()

		if remainingQty.IsZero() {
			return
		}
	}
}

// foldFill folds a (possibly partial) fill of a trade into the currently accumulating bar.
// qty may be less than trade.Quantity when the trade is split across a bar boundary; the
// quote quantity is split in the same proportion.
func (s *VolumeBarStream) foldFill(trade types.Trade, qty fixedpoint.Value) {
	quoteQty := qty
	if !trade.Quantity.IsZero() {
		quoteQty = trade.QuoteQuantity.Mul(qty).Div(trade.Quantity)
	}

	if s.currentVolume.IsZero() {
		s.current = types.KLine{
			Exchange:  trade.Exchange,
			Symbol:    trade.Symbol,
			Interval:  types.Interval(fmt.Sprintf("%svolume", s.barVolume.String())),
			StartTime: trade.Time,
			EndTime:   trade.Time,
			Open:      trade.Price,
			High:      trade.Price,
			Low:       trade.Price,
			Close:     trade.Price,
		}
	} else {
		s.current.EndTime = trade.Time
		s.current.Close = trade.Price
		s.current.High = fixedpoint.Max(s.current.High, trade.Price)
		s.current.Low = fixedpoint.Min(s.current.Low, trade.Price)
	}

	s.current.Volume = s.current.Volume.Add(qty)
	s.current.QuoteVolume = s.current.QuoteVolume.Add(quoteQty)
	if trade.IsBuyer {
		s.current.TakerBuyBaseAssetVolume = s.current.TakerBuyBaseAssetVolume.Add(qty)
		s.current.TakerBuyQuoteAssetVolume = s.current.TakerBuyQuoteAssetVolume.Add(quoteQty)
	}
	s.current.LastTradeID = trade.ID
	s.current.NumberOfTrades++
}

// closeCurrentBar closes, emits, and buffers the currently accumulating bar, then resets the
// accumulator so the next fill starts a fresh bar.
func (s *VolumeBarStream) closeCurrentBar() {
	s.current.Closed = true
	k := s.current
	s.klines = append(s.klines, k)
	s.EmitUpdate(k)

	s.klines = types.ShrinkSlice(s.klines, MaxNumOfVolumeBars, MaxNumOfVolumeBars/5)
	s.currentVolume = fixedpoint.Zero
}

// VolumeBars creates a VolumeBarStream that aggregates market trades of the given symbol from
// source into fixed-volume KLines, emitting a new bar once the accumulated volume reaches
// barVolume.
func VolumeBars(source types.Stream, symbol string, barVolume fixedpoint.Value) *VolumeBarStream {
	s := &VolumeBarStream{barVolume: barVolume}

	source.OnMarketTrade(types.TradeWith(symbol, s.addTrade))

	return s
}
