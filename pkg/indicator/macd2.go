package indicator

import (
	"github.com/c9s/bbgo/pkg/types"
)

/*
NEW INDICATOR DESIGN:

klines := kLines(marketDataStream)
closePrices := closePrices(klines)
macd := MACD(klines, {Fast: 12, Slow: 10})

equals to:

klines := KLines(marketDataStream)
closePrices := ClosePrice(klines)
fastEMA := EMA(closePrices, 7)
slowEMA := EMA(closePrices, 25)
macd := Subtract(fastEMA, slowEMA)
signal := EMA(macd, 16)
histogram := Subtract(macd, signal)
*/

type Float64Source interface {
	types.Series
	OnUpdate(f func(v float64))
}

type Float64Subscription interface {
	types.Series
	AddSubscriber(f func(v float64))
}
