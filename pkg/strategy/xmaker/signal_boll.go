package xmaker

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

var bollingerBandSignalMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_bollinger_band_signal",
		Help: "",
	}, []string{"symbol"})

func init() {
	prometheus.MustRegister(bollingerBandSignalMetrics)
}

type BollingerBandTrendSignal struct {
	types.IntervalWindow
	MinBandWidth float64 `json:"minBandWidth"`
	MaxBandWidth float64 `json:"maxBandWidth"`

	indicator *indicatorv2.BOLLStream
	symbol    string
	lastK     *types.KLine
}

func (s *BollingerBandTrendSignal) Bind(ctx context.Context, session *bbgo.ExchangeSession, symbol string) error {
	if s.MaxBandWidth == 0.0 {
		s.MaxBandWidth = 2.0
	}

	if s.MinBandWidth == 0.0 {
		s.MinBandWidth = 1.0
	}

	s.symbol = symbol
	s.indicator = session.Indicators(symbol).BOLL(s.IntervalWindow, s.MinBandWidth)

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.symbol, s.IntervalWindow.Interval, func(kline types.KLine) {
		s.lastK = &kline
	}))

	bollingerBandSignalMetrics.WithLabelValues(s.symbol).Set(0.0)
	return nil
}

func (s *BollingerBandTrendSignal) CalculateSignal(ctx context.Context) (float64, error) {
	if s.lastK == nil {
		return 0, nil
	}

	closePrice := s.lastK.Close

	// when bid price is lower than the down band, then it's in the downtrend
	// when ask price is higher than the up band, then it's in the uptrend
	lastDownBand := fixedpoint.NewFromFloat(s.indicator.DownBand.Last(0))
	lastUpBand := fixedpoint.NewFromFloat(s.indicator.UpBand.Last(0))

	maxBandWidth := s.indicator.StdDev.Last(0) * s.MaxBandWidth

	signal := 0.0

	// if the price is inside the band, do not vote
	if closePrice.Compare(lastDownBand) > 0 && closePrice.Compare(lastUpBand) < 0 {
		signal = 0.0
	} else if closePrice.Compare(lastDownBand) < 0 {
		signal = lastDownBand.Sub(closePrice).Float64() / maxBandWidth * -2.0
	} else if closePrice.Compare(lastUpBand) > 0 {
		signal = closePrice.Sub(lastUpBand).Float64() / maxBandWidth * 2.0
	}

	log.Infof("[BollingerBandTrendSignal] %f up/down = %f/%f, close price = %f",
		signal,
		lastUpBand.Float64(),
		lastDownBand.Float64(),
		closePrice.Float64())

	bollingerBandSignalMetrics.WithLabelValues(s.symbol).Set(signal)
	return signal, nil
}
