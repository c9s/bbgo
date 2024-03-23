package bbgo

import (
	"github.com/sirupsen/logrus"

	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// IndicatorSet is the v2 standard indicator set
// This will replace StandardIndicator in the future
type IndicatorSet struct {
	Symbol string

	stream types.Stream
	store  *MarketDataStore

	// caches
	kLines      map[types.Interval]*indicatorv2.KLineStream
	closePrices map[types.Interval]*indicatorv2.PriceStream
}

func NewIndicatorSet(symbol string, stream types.Stream, store *MarketDataStore) *IndicatorSet {
	return &IndicatorSet{
		Symbol: symbol,
		store:  store,
		stream: stream,

		kLines:      make(map[types.Interval]*indicatorv2.KLineStream),
		closePrices: make(map[types.Interval]*indicatorv2.PriceStream),
	}
}

func (i *IndicatorSet) KLines(interval types.Interval) *indicatorv2.KLineStream {
	if kLines, ok := i.kLines[interval]; ok {
		return kLines
	}

	kLines := indicatorv2.KLines(i.stream, i.Symbol, interval)
	if kLinesWindow, ok := i.store.KLinesOfInterval(interval); ok {
		kLines.BackFill(*kLinesWindow)
	} else {
		logrus.Warnf("market data store %s kline history not found, unable to backfill the kline stream data", interval)
	}

	i.kLines[interval] = kLines
	return kLines
}

func (i *IndicatorSet) OPEN(interval types.Interval) *indicatorv2.PriceStream {
	return indicatorv2.OpenPrices(i.KLines(interval))
}

func (i *IndicatorSet) HIGH(interval types.Interval) *indicatorv2.PriceStream {
	return indicatorv2.HighPrices(i.KLines(interval))
}

func (i *IndicatorSet) LOW(interval types.Interval) *indicatorv2.PriceStream {
	return indicatorv2.LowPrices(i.KLines(interval))
}

func (i *IndicatorSet) CLOSE(interval types.Interval) *indicatorv2.PriceStream {
	if closePrices, ok := i.closePrices[interval]; ok {
		return closePrices
	}

	closePrices := indicatorv2.ClosePrices(i.KLines(interval))
	i.closePrices[interval] = closePrices
	return closePrices
}

func (i *IndicatorSet) VOLUME(interval types.Interval) *indicatorv2.PriceStream {
	return indicatorv2.Volumes(i.KLines(interval))
}

func (i *IndicatorSet) RSI(iw types.IntervalWindow) *indicatorv2.RSIStream {
	return indicatorv2.RSI2(i.CLOSE(iw.Interval), iw.Window)
}

func (i *IndicatorSet) EMA(iw types.IntervalWindow) *indicatorv2.EWMAStream {
	return i.EWMA(iw)
}

func (i *IndicatorSet) EWMA(iw types.IntervalWindow) *indicatorv2.EWMAStream {
	return indicatorv2.EWMA2(i.CLOSE(iw.Interval), iw.Window)
}

func (i *IndicatorSet) STOCH(iw types.IntervalWindow, dPeriod int) *indicatorv2.StochStream {
	return indicatorv2.Stoch(i.KLines(iw.Interval), iw.Window, dPeriod)
}

func (i *IndicatorSet) BOLL(iw types.IntervalWindow, k float64) *indicatorv2.BOLLStream {
	return indicatorv2.BOLL(i.CLOSE(iw.Interval), iw.Window, k)
}

func (i *IndicatorSet) Keltner(iw types.IntervalWindow, atrLength int) *indicatorv2.KeltnerStream {
	return indicatorv2.Keltner(i.KLines(iw.Interval), iw.Window, atrLength)
}

func (i *IndicatorSet) MACD(interval types.Interval, shortWindow, longWindow, signalWindow int) *indicatorv2.MACDStream {
	return indicatorv2.MACD2(i.CLOSE(interval), shortWindow, longWindow, signalWindow)
}

func (i *IndicatorSet) ATR(interval types.Interval, window int) *indicatorv2.ATRStream {
	return indicatorv2.ATR2(i.KLines(interval), window)
}

func (i *IndicatorSet) ATRP(interval types.Interval, window int) *indicatorv2.ATRPStream {
	return indicatorv2.ATRP2(i.KLines(interval), window)
}

func (i *IndicatorSet) ADX(interval types.Interval, window int) *indicatorv2.ADXStream {
	return indicatorv2.ADX(i.KLines(interval), window)
}
