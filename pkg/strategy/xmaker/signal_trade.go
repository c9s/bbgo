package xmaker

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

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

	session.MarketDataStream.OnMarketTrade(s.handleTrade)
	return nil
}

func (s *TradeVolumeWindowSignal) filterTrades(now time.Time) []types.Trade {
	startTime := now.Add(-time.Duration(s.Window))
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

	trades := s.trades[startIdx:]
	s.trades = trades
	return trades
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
	trades := s.filterTrades(now)
	buyVolume, sellVolume := s.aggTradeVolume(trades)
	totalVolume := buyVolume + sellVolume

	threshold := s.Threshold.Float64()
	buyRatio := buyVolume / totalVolume
	sellRatio := sellVolume / totalVolume

	sig := 0.0
	if buyRatio > threshold {
		sig = (buyRatio - threshold) / 2.0
	} else if sellRatio > threshold {
		sig = -(sellRatio - threshold) / 2.0
	}

	log.Infof("[TradeVolumeWindowSignal] %f buy/sell = %f/%f", sig, buyVolume, sellVolume)

	tradeVolumeWindowSignalMetrics.WithLabelValues(s.symbol).Set(sig)
	return sig, nil
}
