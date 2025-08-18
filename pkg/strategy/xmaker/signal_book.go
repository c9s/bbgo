package xmaker

import (
	"context"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var orderBookSignalMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_order_book_signal",
		Help: "",
	}, []string{"symbol"})

func init() {
	prometheus.MustRegister(orderBookSignalMetrics)
}

type StreamBookSetter interface {
	SetStreamBook(book *types.StreamOrderBook)
}

type OrderBookBestPriceVolumeSignal struct {
	RatioThreshold fixedpoint.Value `json:"ratioThreshold"`
	MinVolume      fixedpoint.Value `json:"minVolume"`
	MinQuoteVolume fixedpoint.Value `json:"minQuoteVolume"`

	symbol string
	book   *types.StreamOrderBook
}

func (s *OrderBookBestPriceVolumeSignal) SetStreamBook(book *types.StreamOrderBook) {
	s.book = book
}

func (s *OrderBookBestPriceVolumeSignal) Bind(ctx context.Context, session *bbgo.ExchangeSession, symbol string) error {
	if s.book == nil {
		return errors.New("s.book can not be nil")
	}

	s.symbol = symbol
	orderBookSignalMetrics.WithLabelValues(s.symbol).Set(0.0)
	return nil
}

func (s *OrderBookBestPriceVolumeSignal) CalculateSignal(ctx context.Context) (float64, error) {
	bid, ask, ok := s.book.BestBidAndAsk()
	if !ok {
		return 0.0, nil
	}

	// TODO: may use scale to define this
	sumVol := bid.Volume.Add(ask.Volume)
	bidRatio := bid.Volume.Div(sumVol)
	askRatio := ask.Volume.Div(sumVol)
	denominator := fixedpoint.One.Sub(s.RatioThreshold)
	signal := 0.0
	if bid.Volume.Compare(s.MinVolume) < 0 && ask.Volume.Compare(s.MinVolume) < 0 {
		signal = 0.0
	} else if bidRatio.Compare(s.RatioThreshold) >= 0 {
		numerator := bidRatio.Sub(s.RatioThreshold)
		signal = numerator.Div(denominator).Float64()
	} else if askRatio.Compare(s.RatioThreshold) >= 0 {
		numerator := askRatio.Sub(s.RatioThreshold)
		signal = -numerator.Div(denominator).Float64()
	}

	log.Infof("[OrderBookBestPriceVolumeSignal] %f bid/ask = %f/%f, bid ratio = %f, ratio threshold = %f",
		signal,
		bid.Volume.Float64(),
		ask.Volume.Float64(),
		bidRatio.Float64(),
		s.RatioThreshold.Float64(),
	)

	orderBookSignalMetrics.WithLabelValues(s.symbol).Set(signal)
	return signal, nil
}
