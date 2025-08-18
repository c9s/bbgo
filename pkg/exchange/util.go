package exchange

import (
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/types"
)

func GetSessionAttributes(exchange types.Exchange) (isMargin, isFutures, isIsolated bool, isolatedSymbol string) {
	if marginExchange, ok := exchange.(types.MarginExchange); ok {
		marginSettings := marginExchange.GetMarginSettings()
		isMargin = marginSettings.IsMargin
		if isMargin {
			isIsolated = marginSettings.IsIsolatedMargin
			if marginSettings.IsIsolatedMargin {
				isolatedSymbol = marginSettings.IsolatedMarginSymbol
			}
		}
	}

	if futuresExchange, ok := exchange.(types.FuturesExchange); ok {
		futuresSettings := futuresExchange.GetFuturesSettings()
		isFutures = futuresSettings.IsFutures
		if isFutures {
			isIsolated = futuresSettings.IsIsolatedFutures
			if futuresSettings.IsIsolatedFutures {
				isolatedSymbol = futuresSettings.IsolatedFuturesSymbol
			}
		}
	}

	return isMargin, isFutures, isIsolated, isolatedSymbol
}

func IsMaxExchange(exchange interface{}) bool {
	_, res := exchange.(*max.Exchange)
	return res
}
