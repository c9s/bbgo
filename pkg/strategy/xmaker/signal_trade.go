package xmaker

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const tradeSliceCapacityLimit = 20000
const tradeSliceShrinkThreshold = tradeSliceCapacityLimit * 4 / 5
const tradeSliceShrinkSize = tradeSliceCapacityLimit * 1 / 5

var tradeVolumeWindowSignalMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_trade_volume_window_signal",
		Help: "",
	}, []string{"symbol"})

func init() {
	prometheus.MustRegister(tradeVolumeWindowSignalMetrics)
}

type TradeVolumeWindowSignal struct {
	Threshold fixedpoint.Value `json:"threshold"`
	Window    types.Duration   `json:"window"`

	trades []types.Trade
	symbol string

	mu sync.Mutex
}

func (s *TradeVolumeWindowSignal) handleTrade(trade types.Trade) {
	s.mu.Lock()
	s.trades = append(s.trades, trade)

	if types.ShrinkSlice(&s.trades, tradeSliceShrinkThreshold, tradeSliceShrinkSize) > 0 {
		runtime.GC()
	}

	s.mu.Unlock()
}

func (s *TradeVolumeWindowSignal) Bind(ctx context.Context, session *bbgo.ExchangeSession, symbol string) error {
	s.symbol = symbol

	if s.Window == 0 {
		s.Window = types.Duration(time.Minute)
	}

	if s.Threshold.IsZero() {
		s.Threshold = fixedpoint.NewFromFloat(0.7)
	}

	s.trades = make([]types.Trade, 0, tradeSliceCapacityLimit)

	session.MarketDataStream.OnMarketTrade(s.handleTrade)
	return nil
}

func (s *TradeVolumeWindowSignal) filterTrades(startTime time.Time) []types.Trade {
	startIdx := 0

	s.mu.Lock()
	defer s.mu.Unlock()

	for idx, td := range s.trades {
		// skip trades before the start time
		if td.Time.Before(startTime) {
			continue
		}

		startIdx = idx
		break
	}

	s.trades = s.trades[startIdx:]
	return s.trades
}

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

func (s *TradeVolumeWindowSignal) CalculateSignal(_ context.Context) (float64, error) {
	now := time.Now()
	trades := s.filterTrades(now.Add(-time.Duration(s.Window)))
	buyVolume, sellVolume := s.aggTradeVolume(trades)
	totalVolume := buyVolume + sellVolume

	threshold := s.Threshold.Float64()
	buyRatio := buyVolume / totalVolume
	sellRatio := sellVolume / totalVolume

	sig := 0.0
	if buyRatio > threshold {
		sig = buyRatio * 2.0
	} else if sellRatio > threshold {
		sig = -sellRatio * 2.0
	}

	log.Infof("[TradeVolumeWindowSignal] %f buy/sell = %f/%f", sig, buyVolume, sellVolume)

	tradeVolumeWindowSignalMetrics.WithLabelValues(s.symbol).Set(sig)
	return sig, nil
}
