package backtest

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

type ExchangeDataSource struct {
	C         chan types.KLine
	Exchange  *Exchange
	Session   *bbgo.ExchangeSession
	Callbacks []func(types.KLine, *ExchangeDataSource)
}
