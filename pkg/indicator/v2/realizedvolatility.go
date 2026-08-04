package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// RealizedVolatilityStream computes a RiskMetrics-style realized volatility from
// a market-trade stream: an exponentially weighted moving average (EWMA) of
// squared trade log-returns.
//
//	rₜ = ln(pₜ / pₜ₋₁)
//	σ²ₜ = λ·σ²ₜ₋₁ + (1−λ)·rₜ²
//	σₜ = sqrt(σ²ₜ)
//
// Unlike a kline/ATR-based volatility, this updates on every trade, so it reacts
// immediately to volatility bursts and decays predictably via the single decay
// factor λ. The emitted value is a dimensionless per-trade return standard
// deviation — comparable in spirit to ATRP (atr/close) but on a per-trade rather
// than per-bar scale, so any downstream thresholds must be calibrated to it.
type RealizedVolatilityStream struct {
	*types.Float64Series

	// lambda is the EWMA decay factor (0 < λ < 1); higher means longer memory.
	lambda float64

	// warmup is the minimum number of returns to accumulate before emitting a
	// value. Until then Last(0) returns 0.0, which downstream consumers treat as
	// the indicator still warming up.
	warmup int

	prevPrice float64
	variance  float64 // running σ²
	count     int     // number of returns seen so far
}

// RealizedVolatility subscribes to a trade stream and emits the per-trade
// realized volatility σ = sqrt(EWMA of squared log-returns). A non-positive or
// out-of-range read (cold stream) returns 0.0 from Last(0).
func RealizedVolatility(source *TradeStream, lambda float64, warmup int) *RealizedVolatilityStream {
	s := &RealizedVolatilityStream{
		Float64Series: types.NewFloat64Series(),
		lambda:        lambda,
		warmup:        warmup,
	}

	source.AddSubscriber(func(trade types.Trade) {
		s.update(trade.Price.Float64())
	})

	return s
}

func (s *RealizedVolatilityStream) update(price float64) {
	// ignore non-positive prices; a log-return is undefined for them.
	if price <= 0 {
		return
	}

	// the first trade only seeds the reference price; no return yet.
	if s.prevPrice <= 0 {
		s.prevPrice = price
		return
	}

	r := math.Log(price / s.prevPrice)
	s.prevPrice = price

	s.variance = s.lambda*s.variance + (1-s.lambda)*r*r
	s.count++

	if s.count < s.warmup {
		return
	}

	s.PushAndEmit(math.Sqrt(s.variance))
	s.Slice = generalTruncate(s.Slice)
}
