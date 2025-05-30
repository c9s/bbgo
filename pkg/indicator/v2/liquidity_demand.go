package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

type MASource interface {
	types.Float64Calculator
	types.Series
}

var (
	_ MASource = &SMAStream{}
	_ MASource = &EWMAStream{}
)

type LiquidityDemandStream struct {
	types.Float64Series

	symbol   string
	interval types.Interval

	openMA, closeMA MASource
	kLineStream     *KLineStream
}

// BackFill fills historical values
func (s *LiquidityDemandStream) BackFill(kLines []types.KLine) {
	if s.IsBackTesting() {
		for _, k := range kLines {
			s.handleKLine(k)
		}
	} else {
		s.kLineStream.BackFill(kLines)
	}
}

func (s *LiquidityDemandStream) IsBackTesting() bool {
	return s.kLineStream == nil
}

func (s *LiquidityDemandStream) handleKLine(k types.KLine) {
	s.openMA.PushAndEmit(k.Open.Float64())
	s.closeMA.PushAndEmit(k.Close.Float64())

	netStrength := s.calculateKLine(k)
	s.PushAndEmit(netStrength)
}

func (s *LiquidityDemandStream) calculateKLine(k types.KLine) float64 {
	high := k.High.Float64()
	low := k.Low.Float64()

	candleRange := high - low
	if candleRange == 0 {
		return 0.0
	}

	openMA := s.openMA.Last(0)
	closeMA := s.closeMA.Last(0)
	// Calculate buy and sell pressure
	var buyStrength, sellStrength float64
	if openMA == 0.0 {
		buyStrength = 1.0
		sellStrength = 0.0
	} else if closeMA == 0.0 {
		buyStrength = 0.0
		sellStrength = 1.0
	} else {
		open := k.Open.Float64()
		close := k.Close.Float64()

		upperShadow := high - math.Max(open, close)
		lowerShadow := math.Min(open, close) - low
		bodyRange := math.Abs(close - open)

		maPriceRatio := closeMA / openMA
		// Calculate buy strength using body size and volume
		bodyStrenth := bodyRange / candleRange
		// Calculate how far closing price pushed above MA
		maStrenth := math.Abs(maPriceRatio - 1.0)
		// MA indicates overall trend context
		if maPriceRatio >= 1.0 {
			// Bullish signal calculations
			// Consider lower shadow as buying pressure from dips
			buyDipStrength := lowerShadow / candleRange
			buyStrength = (bodyStrenth + maStrenth + buyDipStrength) / 3.0
			// Selling pressure is just from upper shadow in bullish candle
			sellStrength = upperShadow / candleRange
		} else {
			// Bearish signal calculations
			// Consider upper shadow as selling pressure from spikes
			sellSpikeStrength := upperShadow / candleRange
			sellStrength = (bodyStrenth + maStrenth + sellSpikeStrength) / 3.0
			// Buying pressure is just from lower shadow in bearish candle
			buyStrength = lowerShadow / candleRange
		}
	}
	volume := k.Volume.Float64()
	logrus.Debugf(
		"Liquidity Demand for %s: openMA=%.4f, closeMA=%.4f, buyStrength=%.4f, sellStrength=%.4f, volume=%.4f",
		s.symbol, openMA, closeMA, buyStrength, sellStrength, volume,
	)
	return (buyStrength - sellStrength) * volume
}

func LiquidityDemand(
	stream types.Stream,
	symbol string,
	interval types.Interval,
	openMA, closeMA MASource,
) *LiquidityDemandStream {
	s := &LiquidityDemandStream{
		Float64Series: *types.NewFloat64Series(),
		symbol:        symbol,
		interval:      interval,
		openMA:        openMA,
		closeMA:       closeMA,
	}

	var kLineStream *KLineStream = nil
	if stream != nil {
		stream.Subscribe(
			types.KLineChannel, symbol, types.SubscribeOptions{
				Interval: interval,
			},
		)
		kLineStream = KLines(stream, symbol, interval)
		kLineStream.AddSubscriber(
			types.KLineWith(symbol, interval, s.handleKLine),
		)
		s.kLineStream = kLineStream
	}
	return s
}
