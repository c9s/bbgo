package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfLiquidityDemand = 3000

type LiquidityDemandStream struct {
	*types.Float64Series

	rawSellDemands, rawBuyDemands *types.Float64Series
	sellDemandMA, buyDemandMA     *SMAStream

	epsilon fixedpoint.Value
}

func (s *LiquidityDemandStream) handleKLine(k types.KLine) {
	netDemand := s.calculateKLine(k)
	s.PushAndEmit(netDemand)
	s.Slice.Truncate(MaxNumOfLiquidityDemand)
}

/*
price_range_buy := high - open
price_range_sell := open - low
buy_demand := volume / price_range_buy iff high - open ≥ ε else 0
sell_demand := volume / price_range_sell iff open - low ≥ ε else 0
buy_demand_ma.Push(buy_demand)
sell_demand_ma.Push(sell_demand)

If the price range is below epsilon, we set demand to 0 to indicate no evidence for that direction.
This prevents division by very small numbers that could result in ±Inf values.
*/
func (s *LiquidityDemandStream) calculateKLine(k types.KLine) float64 {
	priceRangeBuy := k.High.Sub(k.Open)
	priceRangeSell := k.Open.Sub(k.Low)

	var buyDemand, sellDemand float64

	// If buy range is below epsilon, set buy demand to 0 (no evidence for buy demand)
	if priceRangeBuy.Compare(s.epsilon) <= 0 {
		buyDemand = 0
	} else {
		buyDemand = k.Volume.Div(priceRangeBuy).Float64()
	}

	// If sell range is below epsilon, set sell demand to 0 (no evidence for sell demand)
	if priceRangeSell.Compare(s.epsilon) <= 0 {
		sellDemand = 0
	} else {
		sellDemand = k.Volume.Div(priceRangeSell).Float64()
	}

	if math.IsInf(buyDemand, 0) || math.IsNaN(buyDemand) {
		buyDemand = 0
	}
	if math.IsInf(sellDemand, 0) || math.IsNaN(sellDemand) {
		sellDemand = 0
	}

	// push raw buy/sell demands
	s.rawBuyDemands.PushAndEmit(buyDemand)
	s.rawSellDemands.PushAndEmit(sellDemand)
	// get buy/sell moving averages
	bma := s.buyDemandMA.Last(0)
	sma := s.sellDemandMA.Last(0)

	return bma - sma
}

func LiquidityDemand(
	klineStream KLineSubscription,
	iw types.IntervalWindow,
) *LiquidityDemandStream {
	rawSellDemands := types.NewFloat64Series()
	rawBuyDemands := types.NewFloat64Series()
	sellDemandMA := SMA(rawSellDemands, iw.Window)
	buyDemandMA := SMA(rawBuyDemands, iw.Window)

	s := &LiquidityDemandStream{
		Float64Series: types.NewFloat64Series(),

		rawSellDemands: rawSellDemands,
		rawBuyDemands:  rawBuyDemands,
		sellDemandMA:   sellDemandMA,
		buyDemandMA:    buyDemandMA,

		epsilon: fixedpoint.NewFromFloat(1e-5),
	}
	klineStream.AddSubscriber(s.handleKLine)
	return s
}
