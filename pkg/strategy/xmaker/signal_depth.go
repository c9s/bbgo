package xmaker

import (
	"context"
	"math"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var depthRatioSignalMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_depth_ratio_signal",
		Help: "",
	}, []string{"symbol"})

func init() {
	prometheus.MustRegister(depthRatioSignalMetrics)
}

type DepthRatioSignal struct {
	// PriceRange, 2% depth ratio means 2% price range from the mid price
	PriceRange fixedpoint.Value `json:"priceRange"`
	MinRatio   float64          `json:"minRatio"`

	symbol string
	book   *types.StreamOrderBook
}

func (s *DepthRatioSignal) SetStreamBook(book *types.StreamOrderBook) {
	s.book = book
}

func (s *DepthRatioSignal) Bind(ctx context.Context, session *bbgo.ExchangeSession, symbol string) error {
	if s.book == nil {
		return errors.New("s.book can not be nil")
	}

	s.symbol = symbol
	depthRatioSignalMetrics.WithLabelValues(s.symbol).Set(0.0)
	return nil
}

func (s *DepthRatioSignal) CalculateSignal(ctx context.Context) (float64, error) {
	bid, ask, ok := s.book.BestBidAndAsk()
	if !ok {
		return 0.0, nil
	}

	midPrice := bid.Price.Add(ask.Price).Div(fixedpoint.Two)

	asks := s.book.SideBook(types.SideTypeSell)
	bids := s.book.SideBook(types.SideTypeBuy)

	asksInRange := asks.InPriceRange(midPrice, types.SideTypeSell, s.PriceRange)
	bidsInRange := bids.InPriceRange(midPrice, types.SideTypeBuy, s.PriceRange)

	askDepthQuote := asksInRange.SumDepthInQuote()
	bidDepthQuote := bidsInRange.SumDepthInQuote()

	var signal = 0.0

	depthRatio := bidDepthQuote.Div(askDepthQuote.Add(bidDepthQuote))

	// convert ratio into -2.0 and 2.0
	signal = depthRatio.Sub(fixedpoint.NewFromFloat(0.5)).Float64() * 4.0

	// ignore noise
	if math.Abs(signal) < s.MinRatio {
		signal = 0.0
	}

	log.Infof("[DepthRatioSignal] %f bid/ask = %f/%f", signal, bidDepthQuote.Float64(), askDepthQuote.Float64())
	depthRatioSignalMetrics.WithLabelValues(s.symbol).Set(signal)
	return signal, nil
}
