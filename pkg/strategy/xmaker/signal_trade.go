package xmaker

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const tradeSliceCapacityLimit = 10000

var tradeVolumeWindowSignalMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_trade_volume_window_signal",
		Help: "",
	}, []string{"symbol"})

func init() {
	prometheus.MustRegister(tradeVolumeWindowSignalMetrics)
}

// TradeVolumeWindowSignal uses a fixed capacity ring buffer to store trades.
type TradeVolumeWindowSignal struct {
	Threshold fixedpoint.Value `json:"threshold"`
	Window    types.Duration   `json:"window"`

	DecayRate float64 `json:"decayRate"` // decay rate for weighting trades (per second), 0 means no decay

	// new unexported fields for frequency algorithm coefficients:
	//
	// Alpha: represents the volume contribution of each trade. A higher Alpha makes the trade amount
	// more significant in the overall score. Adjust this if you believe trade amounts should have more weight.
	//
	// Beta: represents the frequency contribution of each trade. Each trade contributes a constant value
	// (scaled by decay) to indicate its occurrence. Increase Beta if you want the frequency (number of trades)
	// to have a greater impact on the final signal.
	ConsiderFreq bool    `json:"considerFreq"`
	Alpha        float64 `json:"alpha"` // coefficient for trade volume (default 1.0)
	Beta         float64 `json:"beta"`  // coefficient for trade frequency (default 1.0)

	// Use fixed capacity ring buffer
	trades []types.Trade
	start  int // ring buffer start index
	count  int // current number of stored trades

	symbol string

	mu sync.Mutex
}

// handleTrade adds a trade into the ring buffer.
func (s *TradeVolumeWindowSignal) handleTrade(trade types.Trade) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.count < tradeSliceCapacityLimit {
		// If not full, add trade directly.
		idx := (s.start + s.count) % tradeSliceCapacityLimit
		s.trades[idx] = trade
		s.count++
	} else {
		// If ring buffer is full, overwrite the oldest trade and update start index.
		s.trades[s.start] = trade
		s.start = (s.start + 1) % tradeSliceCapacityLimit
	}
}

// Bind preallocates the fixed capacity ring buffer and binds the market trade callback.
func (s *TradeVolumeWindowSignal) Bind(ctx context.Context, session *bbgo.ExchangeSession, symbol string) error {
	s.symbol = symbol

	if s.Window == 0 {
		s.Window = types.Duration(time.Minute)
	}

	if s.Threshold.IsZero() {
		s.Threshold = fixedpoint.NewFromFloat(0.7)
	}

	// set defaults for frequency coefficients if not provided
	if s.Alpha == 0 {
		s.Alpha = 1.0
	}
	if s.Beta == 0 {
		s.Beta = 1.0
	}

	// Preallocate fixed capacity ring buffer.
	s.trades = make([]types.Trade, tradeSliceCapacityLimit)
	s.start = 0
	s.count = 0

	session.MarketDataStream.OnMarketTrade(s.handleTrade)
	return nil
}

// filterTrades returns trades not before startTime, while updating the ring buffer.
func (s *TradeVolumeWindowSignal) filterTrades(startTime time.Time) []types.Trade {
	s.mu.Lock()
	defer s.mu.Unlock()

	newStart := s.start
	n := s.count
	// Find first trade with time after startTime.
	for i := 0; i < s.count; i++ {
		idx := (s.start + i) % tradeSliceCapacityLimit
		if !s.trades[idx].Time.Before(startTime) {
			newStart = idx
			n = s.count - i
			break
		}
	}
	// Update ring buffer: set start and count.
	s.start = newStart
	s.count = n

	// Copy valid data to a new slice for return.
	res := make([]types.Trade, n, n)
	for i := 0; i < n; i++ {
		idx := (s.start + i) % tradeSliceCapacityLimit
		res[i] = s.trades[idx]
	}
	return res
}

// aggTradeVolume aggregates the buy and sell trade volumes.
func (s *TradeVolumeWindowSignal) aggTradeVolume(trades []types.Trade) (buyVolume, sellVolume float64) {
	for _, td := range trades {
		if td.IsBuyer {
			buyVolume += td.Quantity.Float64()
		} else {
			sellVolume += td.Quantity.Float64()
		}
	}

	return buyVolume, sellVolume
}

// aggDecayedTradeVolume aggregates trade volumes with exponential decay.
// It calculates a weight for each trade as: weight = exp(-DecayRate * deltaSeconds),
// where deltaSeconds is the difference between now and the trade's timestamp.
func aggDecayedTradeVolume(now time.Time, trades []types.Trade, decayRate float64) (buyVolume, sellVolume float64) {
	for _, td := range trades {
		// calculate time difference in seconds
		delta := now.Sub(td.Time.Time()).Seconds()
		weight := math.Exp(-decayRate * delta)
		weightedQty := td.Quantity.Float64() * weight
		if td.IsBuyer {
			buyVolume += weightedQty
		} else {
			sellVolume += weightedQty
		}
	}
	return
}

// calculateSignal computes the signal based on buy and sell volumes.
// If the absolute value of the signal is below the threshold, zero is returned.
func calculateSignal(buyVol, sellVol, threshold float64) float64 {
	total := buyVol + sellVol
	if total == 0 {
		return 0.0
	}
	// signal ranges from -1 to 1.
	sig := (buyVol - sellVol) / total
	if math.Abs(sig) < threshold {
		return 0.0
	}
	return sig
}

// CalculateSignal computes the trading signal using decayed trade volumes.
// If DecayRate > 0, older trades have less influence.
// The computed signal is scaled by 2x.
func (s *TradeVolumeWindowSignal) CalculateSignal(ctx context.Context) (float64, error) {
	if s.ConsiderFreq && s.Alpha != 0.0 && s.Beta != 0.0 {
		return s.CalculateSignalWithFrequency(ctx)
	}

	now := time.Now()
	trades := s.filterTrades(now.Add(-time.Duration(s.Window)))
	var buyVolume, sellVolume float64
	if s.DecayRate > 0 {
		buyVolume, sellVolume = aggDecayedTradeVolume(now, trades, s.DecayRate)
	} else {
		buyVolume, sellVolume = s.aggTradeVolume(trades)
	}
	threshold := s.Threshold.Float64()

	// Use the refined algorithm.
	sig := calculateSignal(buyVolume, sellVolume, threshold)
	sig *= 2.0

	logrus.Infof("[TradeVolumeWindowSignal] signal=%f, buyVolume=%f, sellVolume=%f", sig, buyVolume, sellVolume)
	tradeVolumeWindowSignalMetrics.WithLabelValues(s.symbol).Set(sig)
	return sig, nil
}

// aggDecayedTradeScore aggregates decayed trade volume and frequency scores.
// For each trade, weight = exp(-decayRate * deltaSeconds) if decayRate > 0, else 1.
// The score for each trade is computed as: (Alpha * quantity * weight + Beta * weight),
// then summed separately for buy and sell trades.
func aggDecayedTradeScore(
	now time.Time, trades []types.Trade, decayRate, alpha, beta float64,
) (buyScore, sellScore float64) {
	for _, td := range trades {
		delta := now.Sub(td.Time.Time()).Seconds()
		weight := 1.0
		if decayRate > 0 {
			weight = math.Exp(-decayRate * delta)
		}
		score := alpha*td.Quantity.Float64()*weight + beta*weight
		if td.IsBuyer {
			buyScore += score
		} else {
			sellScore += score
		}
	}
	return
}

// calculateSignalWithFrequency computes the signal using aggregated buy and sell scores.
// It returns 0 if total score is zero or below the threshold.
func calculateSignalWithFrequency(buyScore, sellScore, threshold float64) float64 {
	total := buyScore + sellScore
	if total == 0 {
		return 0.0
	}
	sig := (buyScore - sellScore) / total
	if math.Abs(sig) < threshold {
		return 0.0
	}
	return sig
}

// CalculateSignalWithFrequency computes the trading signal using decayed trade volume and frequency.
// It integrates both the trade amount and the trade occurrence (frequency) using the configured coefficients.
// The final signal is scaled by 2x.
func (s *TradeVolumeWindowSignal) CalculateSignalWithFrequency(_ context.Context) (float64, error) {
	now := time.Now()
	trades := s.filterTrades(now.Add(-time.Duration(s.Window)))
	buyScore, sellScore := aggDecayedTradeScore(now, trades, s.DecayRate, s.Alpha, s.Beta)
	threshold := s.Threshold.Float64()

	sig := calculateSignalWithFrequency(buyScore, sellScore, threshold)
	sig *= 2.0

	logrus.Infof("[TradeVolumeWindowSignal] frequency signal=%f, buyScore=%f, sellScore=%f", sig, buyScore, sellScore)
	tradeVolumeWindowSignalMetrics.WithLabelValues(s.symbol).Set(sig)
	return sig, nil
}
