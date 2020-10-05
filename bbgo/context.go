package bbgo

import (
	"sync"

	"github.com/c9s/bbgo/accounting"
	"github.com/c9s/bbgo/types"
)

type Context struct {
	sync.Mutex

	Symbol string

	// Market is the market configuration of a symbol
	Market types.Market

	AverageBidPrice float64
	CurrentPrice    float64

	Balances                map[string]types.Balance
	ProfitAndLossCalculator *accounting.ProfitAndLossCalculator
	StockManager            *StockManager
}

func (c *Context) SetCurrentPrice(price float64) {
	c.CurrentPrice = price
}
