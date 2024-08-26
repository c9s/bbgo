package circuitbreaker

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	log "github.com/sirupsen/logrus"
)

var consecutiveTotalLossMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_circuit_breaker_consecutive_total_loss",
		Help: "",
	}, []string{"strategy", "strategyInstance"})

var consecutiveLossTimesCounterMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_circuit_breaker_consecutive_loss_times",
		Help: "",
	}, []string{"strategy", "strategyInstance"})

var haltCounterMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_circuit_breaker_halt_counter",
		Help: "",
	}, []string{"strategy", "strategyInstance"})

var haltMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_circuit_breaker_halt",
		Help: "",
	}, []string{"strategy", "strategyInstance"})

var totalProfitMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_circuit_breaker_total_profit",
		Help: "",
	}, []string{"strategy", "strategyInstance"})

var profitWinCounterMetrics = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "bbgo_circuit_breaker_profit_win_counter",
		Help: "profit winning counter",
	}, []string{"strategy", "strategyInstance"})

var profitLossCounterMetrics = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "bbgo_circuit_breaker_profit_loss_counter",
		Help: "profit los counter",
	}, []string{"strategy", "strategyInstance"})

var winningRatioMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_circuit_breaker_winning_ratio",
		Help: "winning ratio",
	}, []string{"strategy", "strategyInstance"})

func init() {
	prometheus.MustRegister(
		consecutiveTotalLossMetrics,
		consecutiveLossTimesCounterMetrics,
		haltCounterMetrics,
		totalProfitMetrics,
		haltMetrics,
		profitWinCounterMetrics,
		profitLossCounterMetrics,
		winningRatioMetrics,
	)
}

type BasicCircuitBreaker struct {
	Enabled bool `json:"enabled"`

	MaximumConsecutiveTotalLoss fixedpoint.Value `json:"maximumConsecutiveTotalLoss"`

	MaximumConsecutiveLossTimes int `json:"maximumConsecutiveLossTimes"`

	IgnoreDustLoss               bool             `json:"ignoreConsecutiveDustLoss"`
	ConsecutiveDustLossThreshold fixedpoint.Value `json:"consecutiveDustLossThreshold"`

	MaximumLossPerRound fixedpoint.Value `json:"maximumLossPerRound"`

	MaximumTotalLoss fixedpoint.Value `json:"maximumTotalLoss"`

	MaximumHaltTimes int `json:"maximumHaltTimes"`

	MaximumHaltTimesExceededPanic bool `json:"maximumHaltTimesExceededPanic"`

	HaltDuration types.Duration `json:"haltDuration"`

	strategyID, strategyInstance string

	haltCounter int
	haltReason  string
	halted      bool

	haltedAt, haltTo time.Time

	// totalProfit is the total PnL, could be negative or positive
	totalProfit          fixedpoint.Value
	consecutiveLossTimes int

	winTimes, lossTimes int
	winRatio            float64

	// consecutiveLoss is a negative number, presents the consecutive loss
	consecutiveLoss fixedpoint.Value

	mu sync.Mutex

	metricsLabels prometheus.Labels
}

func NewBasicCircuitBreaker(strategyID, strategyInstance string) *BasicCircuitBreaker {
	b := &BasicCircuitBreaker{
		Enabled:                       true,
		MaximumConsecutiveLossTimes:   8,
		MaximumHaltTimes:              3,
		MaximumHaltTimesExceededPanic: false,

		HaltDuration:     types.Duration(1 * time.Hour),
		strategyID:       strategyID,
		strategyInstance: strategyInstance,
		metricsLabels:    prometheus.Labels{"strategy": strategyID, "strategyInstance": strategyInstance},
	}

	b.updateMetrics()
	return b
}

func (b *BasicCircuitBreaker) getMetricsLabels() prometheus.Labels {
	if b.metricsLabels != nil {
		return b.metricsLabels
	}

	return prometheus.Labels{"strategy": b.strategyID, "strategyInstance": b.strategyInstance}
}

func (b *BasicCircuitBreaker) updateMetrics() {
	labels := b.getMetricsLabels()
	consecutiveTotalLossMetrics.With(labels).Set(b.consecutiveLoss.Float64())
	consecutiveLossTimesCounterMetrics.With(labels).Set(float64(b.consecutiveLossTimes))
	totalProfitMetrics.With(labels).Set(b.totalProfit.Float64())

	if b.halted {
		haltMetrics.With(labels).Set(1.0)
	} else {
		haltMetrics.With(labels).Set(0.0)
	}

	haltCounterMetrics.With(labels).Set(float64(b.haltCounter))
	winningRatioMetrics.With(labels).Set(b.winRatio)
}

func (b *BasicCircuitBreaker) RecordProfit(profit fixedpoint.Value, now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.totalProfit = b.totalProfit.Add(profit)

	if profit.Sign() < 0 {
		if b.IgnoreDustLoss && profit.Abs().Compare(b.ConsecutiveDustLossThreshold) <= 0 {
			// ignore dust loss
			log.Infof("ignore dust loss (threshold %f): %f", b.ConsecutiveDustLossThreshold.Float64(), profit.Float64())
		} else {
			b.lossTimes++
			b.consecutiveLossTimes++
			b.consecutiveLoss = b.consecutiveLoss.Add(profit)
			profitLossCounterMetrics.With(b.getMetricsLabels()).Inc()
		}
	} else {
		b.winTimes++
		b.consecutiveLossTimes = 0
		b.consecutiveLoss = fixedpoint.Zero
		profitWinCounterMetrics.With(b.getMetricsLabels()).Inc()
	}

	if b.lossTimes == 0 {
		b.winRatio = 999.0
	} else {
		b.winRatio = float64(b.winTimes) / float64(b.lossTimes)
	}

	defer b.updateMetrics()

	if b.MaximumConsecutiveLossTimes > 0 && b.consecutiveLossTimes >= b.MaximumConsecutiveLossTimes {
		b.halt(now, "exceeded MaximumConsecutiveLossTimes")
		return
	}

	if b.MaximumConsecutiveTotalLoss.Sign() > 0 && b.consecutiveLoss.Neg().Compare(b.MaximumConsecutiveTotalLoss) >= 0 {
		b.halt(now, "exceeded MaximumConsecutiveTotalLoss")
		return
	}

	if b.MaximumLossPerRound.Sign() > 0 && profit.Sign() < 0 && profit.Neg().Compare(b.MaximumLossPerRound) >= 0 {
		b.halt(now, "exceeded MaximumLossPerRound")
		return
	}

	// - (-120 [profit]) > 100 [maximum total loss]
	if b.MaximumTotalLoss.Sign() > 0 && b.totalProfit.Neg().Compare(b.MaximumTotalLoss) >= 0 {
		b.halt(now, "exceeded MaximumTotalLoss")
		return
	}
}

func (b *BasicCircuitBreaker) Reset() {
	b.mu.Lock()
	b.reset()
	b.mu.Unlock()
}

func (b *BasicCircuitBreaker) reset() {
	b.halted = false
	b.haltedAt = time.Time{}
	b.haltTo = time.Time{}

	b.totalProfit = fixedpoint.Zero
	b.consecutiveLossTimes = 0
	b.consecutiveLoss = fixedpoint.Zero
	b.updateMetrics()
}

func (b *BasicCircuitBreaker) IsHalted(now time.Time) (string, bool) {
	if !b.Enabled {
		return "disabled", false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.halted {
		return "", false
	}

	// check if it's an expired halt
	if now.After(b.haltTo) {
		b.reset()
		return "", false
	}

	return b.haltReason, true
}

func (b *BasicCircuitBreaker) halt(now time.Time, reason string) {
	b.halted = true
	b.haltCounter++
	b.haltReason = reason
	b.haltedAt = now
	b.haltTo = now.Add(b.HaltDuration.Duration())

	labels := b.getMetricsLabels()
	haltCounterMetrics.With(labels).Set(float64(b.haltCounter))
	haltMetrics.With(labels).Set(1.0)

	defer b.updateMetrics()

	if b.MaximumHaltTimesExceededPanic && b.haltCounter > b.MaximumHaltTimes {
		panic(fmt.Errorf("total %d halt times > maximumHaltTimesExceededPanic %d", b.haltCounter, b.MaximumHaltTimes))
	}
}
