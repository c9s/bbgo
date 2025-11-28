package maxapi

import (
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type KLine struct {
	Symbol                 string
	Interval               string
	StartTime, EndTime     time.Time
	Open, High, Low, Close fixedpoint.Value
	Volume                 fixedpoint.Value
	Closed                 bool
}

func (k KLine) KLine() types.KLine {
	return types.KLine{
		Exchange:  types.ExchangeMax,
		Symbol:    strings.ToUpper(k.Symbol), // global symbol
		Interval:  types.Interval(k.Interval),
		StartTime: types.Time(k.StartTime),
		EndTime:   types.Time(k.EndTime),
		Open:      k.Open,
		Close:     k.Close,
		High:      k.High,
		Low:       k.Low,
		Volume:    k.Volume,
		// QuoteVolume:    util.MustParseFloat(k.QuoteAssetVolume),
		// LastTradeID:    0,
		// NumberOfTrades: k.TradeNum,
		Closed: k.Closed,
	}
}
