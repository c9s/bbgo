package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
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
	*types.Float64Series

	sellDemandMA, buyDemandMA MASource
	kLineStream               *KLineStream
	epsilon                   fixedpoint.Value
}

// BackFill fills historical values
func (s *LiquidityDemandStream) BackFill(kLines []types.KLine) {
	s.kLineStream.BackFill(kLines)
}

func (s *LiquidityDemandStream) handleKLine(k types.KLine) {
	netDemand := s.calculateKLine(k)
	s.PushAndEmit(netDemand)
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
	klineStream *KLineStream,
	sellDemandMA, buyDemandMA MASource,
) *LiquidityDemandStream {
	s := &LiquidityDemandStream{
		Float64Series: types.NewFloat64Series(),
		sellDemandMA:  sellDemandMA,
		buyDemandMA:   buyDemandMA,
		kLineStream:   klineStream,
		epsilon:       fixedpoint.NewFromFloat(1e-6),
	}
	klineStream.OnUpdate(s.handleKLine)
	return s
}
