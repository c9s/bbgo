package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfLiquidityDemand = 3000

type LiquidityDemandStream struct {
	*types.Float64Series

	sellDemandMA, buyDemandMA *SMAStream
	kLineStream               KLineSubscription
	epsilon                   fixedpoint.Value
}

func (s *LiquidityDemandStream) handleKLine(k types.KLine) {
	netDemand := s.calculateKLine(k)
	s.PushAndEmit(netDemand)
	s.Slice.Truncate(MaxNumOfLiquidityDemand)
}

/*
price_range_buy := max(high - open, ε)
price_range_sell := max(open - low, ε)
buy_demand := volume / price_range_buy
sell_demand := volume / price_range_sell
buy_demand_ma.Push(buy_demand)
sell_demand_ma.Push(sell_demand)
*/
func (s *LiquidityDemandStream) calculateKLine(k types.KLine) float64 {
	s.buyDemandMA.PushAndEmit(k.Volume.Div(
		fixedpoint.Max(
			k.High.Sub(k.Open),
			s.epsilon,
		),
	).Float64(),
	)
	s.sellDemandMA.PushAndEmit(
		k.Volume.Div(
			fixedpoint.Max(
				k.Open.Sub(k.Low),
				s.epsilon,
			),
		).Float64(),
	)
	return (s.buyDemandMA.Last(0) - s.sellDemandMA.Last(0))
}

func LiquidityDemand(
	klineStream KLineSubscription,
	sellDemandMA, buyDemandMA *SMAStream,
) *LiquidityDemandStream {
	s := &LiquidityDemandStream{
		Float64Series: types.NewFloat64Series(),
		sellDemandMA:  sellDemandMA,
		buyDemandMA:   buyDemandMA,
		kLineStream:   klineStream,
		epsilon:       fixedpoint.NewFromFloat(1e-6),
	}
	klineStream.AddSubscriber(s.handleKLine)
	return s
}
