package bbgo

import (
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

// IndicatorSet is the v2 standard indicator set
// This will replace StandardIndicator in the future
type IndicatorSet struct {
	Symbol string

	stream types.Stream
	store  *MarketDataStore

	// caches
	kLines      map[types.Interval]*indicator.KLineStream
	closePrices map[types.Interval]*indicator.PriceStream
}

func NewIndicatorSet(symbol string, stream types.Stream, store *MarketDataStore) *IndicatorSet {
	return &IndicatorSet{
		Symbol: symbol,
		store:  store,
		stream: stream,

		kLines:      make(map[types.Interval]*indicator.KLineStream),
		closePrices: make(map[types.Interval]*indicator.PriceStream),
	}
}

func (i *IndicatorSet) KLines(interval types.Interval) *indicator.KLineStream {
	if kLines, ok := i.kLines[interval]; ok {
		return kLines
	}

	kLines := indicator.KLines(i.stream, i.Symbol, interval)
	if kLinesWindow, ok := i.store.KLinesOfInterval(interval); ok {
		kLines.BackFill(*kLinesWindow)
	} else {
		logrus.Warnf("market data store %s kline history not found, unable to backfill the kline stream data", interval)
	}

	i.kLines[interval] = kLines
	return kLines
}

func (i *IndicatorSet) OPEN(interval types.Interval) *indicator.PriceStream {
	return indicator.OpenPrices(i.KLines(interval))
}

func (i *IndicatorSet) HIGH(interval types.Interval) *indicator.PriceStream {
	return indicator.HighPrices(i.KLines(interval))
}

func (i *IndicatorSet) LOW(interval types.Interval) *indicator.PriceStream {
	return indicator.LowPrices(i.KLines(interval))
}

func (i *IndicatorSet) CLOSE(interval types.Interval) *indicator.PriceStream {
	if closePrices, ok := i.closePrices[interval]; ok {
		return closePrices
	}

	closePrices := indicator.ClosePrices(i.KLines(interval))
	i.closePrices[interval] = closePrices
	return closePrices
}

func (i *IndicatorSet) VOLUME(interval types.Interval) *indicator.PriceStream {
	return indicator.Volumes(i.KLines(interval))
}

func (i *IndicatorSet) RSI(iw types.IntervalWindow) *indicator.RSIStream {
	return indicator.RSI2(i.CLOSE(iw.Interval), iw.Window)
}

func (i *IndicatorSet) EMA(iw types.IntervalWindow) *indicator.EWMAStream {
	return i.EWMA(iw)
}

func (i *IndicatorSet) EWMA(iw types.IntervalWindow) *indicator.EWMAStream {
	return indicator.EWMA2(i.CLOSE(iw.Interval), iw.Window)
}

func (i *IndicatorSet) STOCH(iw types.IntervalWindow, dPeriod int) *indicator.StochStream {
	return indicator.Stoch2(i.KLines(iw.Interval), iw.Window, dPeriod)
}

func (i *IndicatorSet) BOLL(iw types.IntervalWindow, k float64) *indicator.BOLLStream {
	return indicator.BOLL2(i.CLOSE(iw.Interval), iw.Window, k)
}

func (i *IndicatorSet) MACD(interval types.Interval, shortWindow, longWindow, signalWindow int) *indicator.MACDStream {
	return indicator.MACD2(i.CLOSE(interval), shortWindow, longWindow, signalWindow)
}

func (i *IndicatorSet) ATR(interval types.Interval, window int) *indicator.ATRStream {
	return indicator.ATR2(i.KLines(interval), window)
}

func (i *IndicatorSet) ATRP(interval types.Interval, window int) *indicator.ATRPStream {
	return indicator.ATRP2(i.KLines(interval), window)
}
