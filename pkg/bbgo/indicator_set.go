package bbgo

import (
	"github.com/sirupsen/logrus"

	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/indicator/v2/momentum"
	"github.com/c9s/bbgo/pkg/indicator/v2/trend"
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

func (i *IndicatorSet) RSI(iw types.IntervalWindow) *momentum.RSIStream {
	return momentum.RSI2(i.CLOSE(iw.Interval), iw.Window)
}

func (i *IndicatorSet) EMA(iw types.IntervalWindow) *trend.EWMAStream {
	return i.EWMA(iw)
}

func (i *IndicatorSet) EWMA(iw types.IntervalWindow) *trend.EWMAStream {
	return trend.EWMA2(i.CLOSE(iw.Interval), iw.Window)
}

func (i *IndicatorSet) STOCH(iw types.IntervalWindow, dPeriod int) *indicatorv2.StochStream {
	return indicatorv2.Stoch(i.KLines(iw.Interval), iw.Window, dPeriod)
}

func (i *IndicatorSet) BOLL(iw types.IntervalWindow, k float64) *indicatorv2.BOLLStream {
	return indicatorv2.BOLL(i.CLOSE(iw.Interval), iw.Window, k)
}

func (i *IndicatorSet) MACD(interval types.Interval, shortWindow, longWindow, signalWindow int) *trend.MACDStream {
	return trend.MACD2(i.CLOSE(interval), shortWindow, longWindow, signalWindow)
}

func (i *IndicatorSet) ATR(interval types.Interval, window int) *trend.ATRStream {
	return trend.ATR2(i.KLines(interval), window)
}

func (i *IndicatorSet) ATRP(interval types.Interval, window int) *trend.ATRPStream {
	return trend.ATRP2(i.KLines(interval), window)
}
