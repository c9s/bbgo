package bbgo

import "github.com/c9s/bbgo/pkg/bbgo/types"

type TradingContext struct {
	KLineWindowSize int
	KLineWindows    map[string]types.KLineWindow

	Symbol          string

	// Market is the market configuration of a symbol
	Market 			Market

	AverageBidPrice float64
	CurrentPrice    float64

	ProfitAndLossCalculator *ProfitAndLossCalculator
}

func (c *TradingContext) SetCurrentPrice(price float64) {
	c.CurrentPrice = price
	c.ProfitAndLossCalculator.SetCurrentPrice(price)
}

func (c *TradingContext) AddKLine(kline types.KLine) types.KLineWindow {
	var klineWindow = c.KLineWindows[kline.Interval]
	klineWindow.Add(kline)

	if c.KLineWindowSize > 0 {
		klineWindow.Truncate(c.KLineWindowSize)
	}

	return klineWindow
}


