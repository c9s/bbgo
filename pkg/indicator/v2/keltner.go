package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type KeltnerStream struct {
	types.SeriesBase

	EWMA   *EWMAStream
	StdDev *StdDevStream
	ATR    *ATRStream

	highPrices, lowPrices, closePrices *PriceStream

	Mid                              *types.Float64Series
	FirstUpperBand, FirstLowerBand   *types.Float64Series
	SecondUpperBand, SecondLowerBand *types.Float64Series
	ThirdUpperBand, ThirdLowerBand   *types.Float64Series
}

func Keltner(source KLineSubscription, window, atrLength int, atrMultipliers ...float64) *KeltnerStream {
	atr := ATR2(source, atrLength)

	highPrices := HighPrices(source)
	lowPrices := LowPrices(source)
	closePrices := ClosePrices(source)
	ewma := EWMA2(closePrices, window)

	s := &KeltnerStream{
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

	var firstMultiplier, secondMultiplier, thirdMultiplier float64 = 1, 2, 3
	if len(atrMultipliers) > 0 {
		firstMultiplier = atrMultipliers[0]
	}
	if len(atrMultipliers) > 1 {
		secondMultiplier = atrMultipliers[1]
	}
	if len(atrMultipliers) > 2 {
		thirdMultiplier = atrMultipliers[2]
	}

	source.AddSubscriber(func(kLine types.KLine) {
		mid := s.EWMA.Last(0)
		atr := s.ATR.Last(0)
		s.Mid.PushAndEmit(mid)
		s.FirstUpperBand.PushAndEmit(mid + firstMultiplier*atr)
		s.FirstLowerBand.PushAndEmit(mid - firstMultiplier*atr)
		s.SecondUpperBand.PushAndEmit(mid + secondMultiplier*atr)
		s.SecondLowerBand.PushAndEmit(mid - secondMultiplier*atr)
		s.ThirdUpperBand.PushAndEmit(mid + thirdMultiplier*atr)
		s.ThirdLowerBand.PushAndEmit(mid - thirdMultiplier*atr)
	})
	return s
}
