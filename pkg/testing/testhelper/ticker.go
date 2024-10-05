package testhelper

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var _tickers = map[string]types.Ticker{
	"BTCUSDT": {
		Time:   time.Now(),
		Volume: fixedpoint.Zero,
		Last:   fixedpoint.NewFromFloat(19000.0),
		Open:   fixedpoint.NewFromFloat(19500.0),
		High:   fixedpoint.NewFromFloat(19900.0),
		Low:    fixedpoint.NewFromFloat(18800.0),
		Buy:    fixedpoint.NewFromFloat(19500.0),
		Sell:   fixedpoint.NewFromFloat(18900.0),
	},

	"ETHUSDT": {
		Time:   time.Now(),
		Volume: fixedpoint.Zero,
		Open:   fixedpoint.NewFromFloat(2510.0),
		High:   fixedpoint.NewFromFloat(2530.0),
		Low:    fixedpoint.NewFromFloat(2505.0),
		Last:   fixedpoint.NewFromFloat(2520.0),
		Buy:    fixedpoint.NewFromFloat(2519.0),
		Sell:   fixedpoint.NewFromFloat(2521.0),
	},

	"USDTTWD": {
		Time:   time.Now(),
		Volume: fixedpoint.Zero,
		Open:   fixedpoint.NewFromFloat(32.1),
		High:   fixedpoint.NewFromFloat(32.31),
		Low:    fixedpoint.NewFromFloat(32.01),
		Last:   fixedpoint.NewFromFloat(32.0),
		Buy:    fixedpoint.NewFromFloat(32.0),
		Sell:   fixedpoint.NewFromFloat(32.01),
	},
}

func Ticker(symbol string) types.Ticker {
	ticker, ok := _tickers[symbol]
	if !ok {
		panic(fmt.Errorf("%s test ticker not found, valid tickers: %+v", symbol, []string{"BTCUSDT", "ETHUSDT", "USDTTWD"}))
	}

	return ticker
}
