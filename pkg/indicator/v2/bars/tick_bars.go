package bars

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfTickBars = 5_000

// TickBarStream aggregates trades of a symbol into fixed-tick-count KLines ("tick bars"):
// every numTicks trades closes the current bar and starts a new one.
//
//go:generate callbackgen -type TickBarStream
type TickBarStream struct {
	updateCallbacks []func(k types.KLine)

	numTicks int
	klines   []types.KLine

	current      types.KLine
	currentTicks int
}

func (s *TickBarStream) Length() int {
	return len(s.klines)
}

func (s *TickBarStream) Last(i int) *types.KLine {
	l := len(s.klines)
	if i < 0 || l-1-i < 0 {
		return nil
	}

	return &s.klines[l-1-i]
}

func (s *TickBarStream) Tail(n int) types.KLineWindow {
	l := len(s.klines)
	if n >= l {
		return s.klines
	}

	return s.klines[l-n : l]
}

// AddSubscriber adds the subscriber function and pushes historical tick bars to the subscriber
func (s *TickBarStream) AddSubscriber(f func(k types.KLine)) {
	s.OnUpdate(f)

	if len(s.klines) == 0 {
		return
	}

	// push historical tick bars to the subscriber
	for _, k := range s.klines {
		f(k)
	}
}

// addTrade folds a single trade into the currently accumulating bar, closing and emitting it
// once numTicks trades have been collected.
func (s *TickBarStream) addTrade(trade types.Trade) {
	if s.currentTicks == 0 {
		s.current = types.KLine{
			Exchange:  trade.Exchange,
			Symbol:    trade.Symbol,
			Interval:  types.Interval(fmt.Sprintf("%dticks", s.numTicks)),
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

	s.current.Volume = s.current.Volume.Add(trade.Quantity)
	s.current.QuoteVolume = s.current.QuoteVolume.Add(trade.QuoteQuantity)
	if trade.IsBuyer {
		s.current.TakerBuyBaseAssetVolume = s.current.TakerBuyBaseAssetVolume.Add(trade.Quantity)
		s.current.TakerBuyQuoteAssetVolume = s.current.TakerBuyQuoteAssetVolume.Add(trade.QuoteQuantity)
	}
	s.current.LastTradeID = trade.ID
	s.current.NumberOfTrades++
	s.currentTicks++

	if s.currentTicks < s.numTicks {
		return
	}

	s.current.Closed = true
	k := s.current
	s.klines = append(s.klines, k)
	s.EmitUpdate(k)

	s.klines = types.ShrinkSlice(s.klines, MaxNumOfTickBars, MaxNumOfTickBars/5)
	s.currentTicks = 0
}

// TickBars creates a TickBarStream that aggregates market trades of the given symbol from
// source into fixed-tick-count KLines, emitting a new bar every numTicks trades.
func TickBars(source types.Stream, symbol string, numTicks int) *TickBarStream {
	s := &TickBarStream{numTicks: numTicks}

	source.OnMarketTrade(types.TradeWith(symbol, s.addTrade))

	return s
}
