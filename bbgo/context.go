package bbgo

import "time"

type TradingContext struct {
	KLineWindowSize int
	KLineWindows    map[string]KLineWindow
	AverageBidPrice float64
	Stock           float64
	Profit          float64
	CurrentPrice    float64
	Trades          []Trade
	TradeStartTime  time.Time
	Symbol          string
}

func (c *TradingContext) AddKLine(kline KLine) KLineWindow {
	var klineWindow = c.KLineWindows[kline.Interval]
	klineWindow.Add(kline)

	if c.KLineWindowSize > 0 {
		klineWindow.Truncate(c.KLineWindowSize)
	}

	return klineWindow
}

func (c *TradingContext) UpdatePnL() {
	c.AverageBidPrice, c.Stock, c.Profit, _ = CalculateCostAndProfit(c.Trades, c.CurrentPrice)
}

