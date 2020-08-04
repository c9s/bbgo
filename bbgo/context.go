package bbgo

import (
	"github.com/c9s/bbgo/pkg/bbgo/types"
	"sync"
)

type TradingContext struct {
	sync.Mutex

	Symbol string

	// Market is the market configuration of a symbol
	Market types.Market

	AverageBidPrice float64
	CurrentPrice    float64

	Balances                map[string]types.Balance
	Quota                   map[string]types.Balance
	ProfitAndLossCalculator *ProfitAndLossCalculator
	StockManager            *StockManager
}

func (c *TradingContext) SetCurrentPrice(price float64) {
	c.CurrentPrice = price
}
