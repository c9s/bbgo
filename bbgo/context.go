package bbgo

import "github.com/c9s/bbgo/pkg/bbgo/types"

type TradingContext struct {
	Symbol          string

	// Market is the market configuration of a symbol
	Market types.Market

	AverageBidPrice float64
	CurrentPrice    float64

	ProfitAndLossCalculator *ProfitAndLossCalculator
}

func (c *TradingContext) SetCurrentPrice(price float64) {
	c.CurrentPrice = price
	c.ProfitAndLossCalculator.SetCurrentPrice(price)
}



