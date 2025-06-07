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

func (s *LiquidityDemandStream) calculateKLine(k types.KLine) float64 {
	if k.Close.Compare(k.Open) > 0 {
		priceRange := k.High.Sub(k.Open).Add(s.epsilon)
		buyDemand := k.Volume.Div(priceRange).Float64()
		s.buyDemandMA.PushAndEmit(buyDemand)
		s.sellDemandMA.PushAndEmit(0.0) // decay
	} else {
		priceRange := k.Open.Sub(k.Low).Add(s.epsilon)
		sellDemand := k.Volume.Div(priceRange).Float64()
		s.sellDemandMA.PushAndEmit(sellDemand)
		s.buyDemandMA.PushAndEmit(0.0) // decay
	}
	buyDemand := s.buyDemandMA.Last(0)
	sellDemand := s.sellDemandMA.Last(0)
	return buyDemand - sellDemand
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
