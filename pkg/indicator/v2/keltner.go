package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type KeltnerStream struct {
	types.SeriesBase

	window, atrLength int

	EWMA   *EWMAStream
	StdDev *StdDevStream
	ATR    *ATRStream

	highPrices, lowPrices, closePrices *PriceStream

	Mid                              *types.Float64Series
	FirstUpperBand, FirstLowerBand   *types.Float64Series
	SecondUpperBand, SecondLowerBand *types.Float64Series
	ThirdUpperBand, ThirdLowerBand   *types.Float64Series
}

func Keltner(source KLineSubscription, window, atrLength int) *KeltnerStream {
	atr := ATR2(source, atrLength)

	highPrices := HighPrices(source)
	lowPrices := LowPrices(source)
	closePrices := ClosePrices(source)
	ewma := EWMA2(closePrices, window)

	s := &KeltnerStream{
		window:          window,
		atrLength:       atrLength,
		highPrices:      highPrices,
		lowPrices:       lowPrices,
		closePrices:     closePrices,
		ATR:             atr,
		EWMA:            ewma,
		Mid:             types.NewFloat64Series(),
		FirstUpperBand:  types.NewFloat64Series(),
		FirstLowerBand:  types.NewFloat64Series(),
		SecondUpperBand: types.NewFloat64Series(),
		SecondLowerBand: types.NewFloat64Series(),
		ThirdUpperBand:  types.NewFloat64Series(),
		ThirdLowerBand:  types.NewFloat64Series(),
	}

	source.AddSubscriber(func(kLine types.KLine) {
		mid := s.EWMA.Last(0)
		atr := s.ATR.Last(0)
		s.Mid.PushAndEmit(mid)
		s.FirstUpperBand.PushAndEmit(mid + atr)
		s.FirstLowerBand.PushAndEmit(mid - atr)
		s.SecondUpperBand.PushAndEmit(mid + 2*atr)
		s.SecondLowerBand.PushAndEmit(mid - 2*atr)
		s.ThirdUpperBand.PushAndEmit(mid + 3*atr)
		s.ThirdLowerBand.PushAndEmit(mid - 3*atr)
	})
	return s
}
