package indicatorv2

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfKLines = 5_000

var (
	metricsStreamKLinePrices = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_stream_kline_ohlc",
			Help: "the open, high, low and close price of the kline",
		}, []string{"exchange", "symbol", "interval", "type"},
	)

	metricsStreamKLineVolume = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_stream_kline_volume",
			Help: "the volume of the kline",
		}, []string{"exchange", "symbol", "interval"},
	)
)

func init() {
	prometheus.MustRegister(
		metricsStreamKLinePrices,
		metricsStreamKLineVolume,
	)
}

//go:generate callbackgen -type KLineStream
type KLineStream struct {
	updateCallbacks []func(k types.KLine)

	kLines []types.KLine
}

func (s *KLineStream) Length() int {
	return len(s.kLines)
}

func (s *KLineStream) Last(i int) *types.KLine {
	l := len(s.kLines)
	if i < 0 || l-1-i < 0 {
		return nil
	}

	return &s.kLines[l-1-i]
}

func (s *KLineStream) Tail(n int) types.KLineWindow {
	l := len(s.kLines)
	if n >= l {
		return s.kLines
	}

	return s.kLines[l-n : l]
}

// AddSubscriber adds the subscriber function and push historical data to the subscriber
func (s *KLineStream) AddSubscriber(f func(k types.KLine)) {
	s.OnUpdate(f)

	if len(s.kLines) == 0 {
		return
	}

	// push historical klines to the subscriber
	for _, k := range s.kLines {
		f(k)
	}
}

func (s *KLineStream) BackFill(kLines []types.KLine) {
	for _, k := range kLines {
		s.kLines = append(s.kLines, k)
		s.EmitUpdate(k)
	}
}

func (s *KLineStream) metricsKLineUpdater(k types.KLine) {
	labels := prometheus.Labels{
		"exchange": k.Exchange.String(),
		"symbol":   k.Symbol,
		"interval": k.Interval.String(),
	}

	metricsStreamKLinePrices.MustCurryWith(labels).With(prometheus.Labels{"type": "open"}).Set(k.Open.Float64())
	metricsStreamKLinePrices.MustCurryWith(labels).With(prometheus.Labels{"type": "close"}).Set(k.Close.Float64())
	metricsStreamKLinePrices.MustCurryWith(labels).With(prometheus.Labels{"type": "high"}).Set(k.High.Float64())
	metricsStreamKLinePrices.MustCurryWith(labels).With(prometheus.Labels{"type": "low"}).Set(k.Low.Float64())

	metricsStreamKLineVolume.With(labels).Set(k.Volume.Float64())
}

// KLines creates a KLine stream that pushes the klines to the subscribers
func KLines(source types.Stream, symbol string, interval types.Interval) *KLineStream {
	s := &KLineStream{}

	source.OnKLineClosed(types.KLineWith(symbol, interval, func(k types.KLine) {
		s.kLines = append(s.kLines, k)
		s.EmitUpdate(k)

		if viper.GetBool("metrics") {
			s.metricsKLineUpdater(k)
		}

		s.kLines = types.ShrinkSlice(s.kLines, MaxNumOfKLines, MaxNumOfKLines/5)
	}))

	return s
}

type KLineSubscription interface {
	AddSubscriber(f func(k types.KLine))
	Length() int
	Last(i int) *types.KLine
}
