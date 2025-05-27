package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type PremiumType int

const (
	Neutral  PremiumType = iota // no premium
	Premium                     // Base/Quote1 > Base/Quote2
	Discount                    // Base/Quote1 < Base/Quote2
)

type PremiumSignalStream struct {
	*types.Float64Series

	kLineStream      *KLineStream
	symbol1, symbol2 string
	interval         types.Interval
	price1, price2   float64
	premiumMargin    float64
}

func (s *PremiumSignalStream) BackFill(klines []types.KLine) {
	if s.IsBackTesting() {
		update1 := types.KLineWith(s.symbol1, s.interval, func(kline types.KLine) {
			s.price1 = kline.Close.Float64()
		})
		update2 := types.KLineWith(s.symbol2, s.interval, func(kline types.KLine) {
			s.price2 = kline.Close.Float64()
		})
		for _, kline := range klines {
			update1(kline)
			update2(kline)
			s.calculatePremium()
		}
	} else {
		s.kLineStream.BackFill(klines)
	}
}

func (s *PremiumSignalStream) IsBackTesting() bool {
	return s.kLineStream == nil
}

func (s *PremiumSignalStream) calculatePremium() {
	if s.price1 == 0 || s.price2 == 0 {
		return
	}
	// calculate the premium type based on the prices
	var premiumType PremiumType
	if s.price1 > (1.0+s.premiumMargin)*s.price2 {
		premiumType = Premium
	} else if s.price1 < (1.0-s.premiumMargin)*s.price2 {
		premiumType = Discount
	} else {
		premiumType = Neutral
	}
	// push and emit
	s.PushAndEmit(float64(premiumType))
}

func PremiumSignal(
	stream types.Stream,
	baseCurrency string,
	quoteCurrency1 string,
	quoteCurrency2 string,
	interval types.Interval,
	premiumMargin float64,
) *PremiumSignalStream {
	s := &PremiumSignalStream{
		Float64Series: types.NewFloat64Series(),
		premiumMargin: premiumMargin,
		interval:      interval,
		symbol1:       baseCurrency + quoteCurrency1,
		symbol2:       baseCurrency + quoteCurrency2,
	}
	// stream can be nil (ex: backtesting)
	if stream != nil {
		stream.Subscribe(
			types.KLineChannel,
			s.symbol1,
			types.SubscribeOptions{
				Interval: s.interval,
			},
		)
		stream.Subscribe(
			types.KLineChannel,
			s.symbol2,
			types.SubscribeOptions{
				Interval: s.interval,
			},
		)

		s.kLineStream = KLines(stream, s.symbol1, s.interval)
		s.kLineStream.AddSubscriber(
			types.KLineWith(s.symbol1, s.interval, func(kline types.KLine) {
				s.price1 = kline.Close.Float64()
				s.calculatePremium()
			}),
		)
		s.kLineStream.AddSubscriber(
			types.KLineWith(s.symbol2, s.interval, func(kline types.KLine) {
				s.price2 = kline.Close.Float64()
				s.calculatePremium()
			}),
		)
	}
	return s
}
