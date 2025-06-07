package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
)

const MaxNumOfKLines = 5_000

var (
	metricsKLineStreamOpen = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_kline_stream_open",
			Help: "open price of the kline",
		}, []string{"exchange", "symbol", "interval", "startTime", "endTime"},
	)

	metricsKLineStreamClose = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_kline_stream_close",
			Help: "close price of the kline",
		}, []string{"exchange", "symbol", "interval", "startTime", "endTime"},
	)

	metricsKLineStreamHigh = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_kline_stream_high",
			Help: "high price of the kline",
		}, []string{"exchange", "symbol", "interval", "startTime", "endTime"},
	)

	metricsKLineStreamLow = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_kline_stream_low",
			Help: "low price of the kline",
		}, []string{"exchange", "symbol", "interval", "startTime", "endTime"},
	)
)

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
		"exchange":  k.Exchange.String(),
		"symbol":    k.Symbol,
		"interval":  k.Interval.String(),
		"startTime": k.StartTime.Time().Format("2006-01-02 15:04:05"),
		"endTime":   k.EndTime.Time().Format("2006-01-02 15:04:05"),
	}

	metricsKLineStreamOpen.With(labels).Set(k.Open.Float64())
	metricsKLineStreamClose.With(labels).Set(k.Close.Float64())
	metricsKLineStreamHigh.With(labels).Set(k.High.Float64())
	metricsKLineStreamLow.With(labels).Set(k.Low.Float64())
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
