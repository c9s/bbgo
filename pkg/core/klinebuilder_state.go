package core

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// kLineBuilderState builds klines from trades and trades only
type kLineBuilderState struct {
	symbol string

	// kLinesMap is a map from interval to accumulated kline of that interval
	lastKLinesMap, kLinesMap map[types.Interval]*types.KLine
}

func NewKLineBuilderState(symbol string) *kLineBuilderState {
	return &kLineBuilderState{
		symbol:        symbol,
		lastKLinesMap: make(map[types.Interval]*types.KLine),
		kLinesMap:     make(map[types.Interval]*types.KLine),
	}
}

// Reset will reset the kline of the given interval to a new open kline with the given start time
// It's expected that this method is called when the kline is closed
func (kb *kLineBuilderState) Reset(interval types.Interval, startTime types.Time) {
	if lastKLine, ok := kb.kLinesMap[interval]; ok {
		kb.lastKLinesMap[interval] = lastKLine
	}
	kb.kLinesMap[interval] = &types.KLine{
		Symbol:    kb.symbol,
		Interval:  interval,
		StartTime: startTime,
		Closed:    false,
	}
}

func (kb *kLineBuilderState) GetKLine(interval types.Interval) (*types.KLine, bool) {
	kline, ok := kb.kLinesMap[interval]
	return kline, ok
}

func (kb *kLineBuilderState) GetLastKLine(interval types.Interval) (*types.KLine, bool) {
	lastKLine, ok := kb.lastKLinesMap[interval]
	return lastKLine, ok
}

func (kb *kLineBuilderState) AddTrade(trade types.Trade) {
	// TODO: fix timezone issue of the trade
	// the time of the trade is in the time zone of the exchange
	// however, the kline is build in the local time zone
	if trade.Symbol != kb.symbol {
		return
	}
	klineDelta := types.KLine{
		Symbol:      trade.Symbol,
		Open:        trade.Price,
		Close:       trade.Price,
		High:        trade.Price,
		Low:         trade.Price,
		EndTime:     types.Time(time.Now()), // use current time to fix the timezone issue (temporary)
		Volume:      trade.Quantity,
		QuoteVolume: trade.QuoteQuantity,
	}
	for _, oldKline := range kb.kLinesMap {
		updateKlineInPlace(oldKline, &klineDelta)
	}

}

func updateKlineInPlace(existing, delta *types.KLine) {
	existing.Close = delta.Close
	existing.High = fixedpoint.Max(existing.High, delta.High)
	if existing.NumberOfTrades == 0 {
		existing.Open = delta.Open
		existing.Low = delta.Low
	} else {
		existing.Low = fixedpoint.Min(existing.Low, delta.Low)

	}
	existing.EndTime = delta.EndTime
	existing.Volume = existing.Volume.Add(delta.Volume)
	existing.QuoteVolume = existing.QuoteVolume.Add(delta.QuoteVolume)
	existing.NumberOfTrades++
}
