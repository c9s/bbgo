package indicatorv2

import (
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestKLineStream_metricsKLineUpdater(t *testing.T) {
	// Create test KLine data
	now := time.Now()
	k := types.KLine{
		Exchange:  types.ExchangeBinance,
		Symbol:    "BTCUSDT",
		StartTime: types.Time(now),
		EndTime:   types.Time(now.Add(time.Minute)),
		Interval:  types.Interval1m,
		Open:      fixedpoint.NewFromFloat(50000.0),
		Close:     fixedpoint.NewFromFloat(51000.0),
		High:      fixedpoint.NewFromFloat(52000.0),
		Low:       fixedpoint.NewFromFloat(49000.0),
		Volume:    fixedpoint.NewFromFloat(100.5),
	}

	// Create KLineStream instance
	stream := &KLineStream{}

	// Call the function being tested
	stream.metricsKLineUpdater(k)

	// Base labels for metrics
	baseLabels := prometheus.Labels{
		"exchange": k.Exchange.String(),
		"symbol":   k.Symbol,
		"interval": k.Interval.String(),
	}

	// Test OHLC price metrics
	// Test open price
	openLabels := prometheus.Labels{
		"exchange": k.Exchange.String(),
		"symbol":   k.Symbol,
		"interval": k.Interval.String(),
		"type":     "open",
	}
	openValue, err := metricsStreamKLinePrices.GetMetricWith(openLabels)
	assert.NoError(t, err)
	assert.Equal(t, 50000.0, getGaugeValue(t, openValue))

	// Test close price
	closeLabels := prometheus.Labels{
		"exchange": k.Exchange.String(),
		"symbol":   k.Symbol,
		"interval": k.Interval.String(),
		"type":     "close",
	}
	closeValue, err := metricsStreamKLinePrices.GetMetricWith(closeLabels)
	assert.NoError(t, err)
	assert.Equal(t, 51000.0, getGaugeValue(t, closeValue))

	// Test high price
	highLabels := prometheus.Labels{
		"exchange": k.Exchange.String(),
		"symbol":   k.Symbol,
		"interval": k.Interval.String(),
		"type":     "high",
	}
	highValue, err := metricsStreamKLinePrices.GetMetricWith(highLabels)
	assert.NoError(t, err)
	assert.Equal(t, 52000.0, getGaugeValue(t, highValue))

	// Test low price
	lowLabels := prometheus.Labels{
		"exchange": k.Exchange.String(),
		"symbol":   k.Symbol,
		"interval": k.Interval.String(),
		"type":     "low",
	}
	lowValue, err := metricsStreamKLinePrices.GetMetricWith(lowLabels)
	assert.NoError(t, err)
	assert.Equal(t, 49000.0, getGaugeValue(t, lowValue))

	// Test volume metric
	volumeValue, err := metricsStreamKLineVolume.GetMetricWith(baseLabels)
	assert.NoError(t, err)
	assert.Equal(t, 100.5, getGaugeValue(t, volumeValue))
}

// Helper function: Get the value of a Gauge metric
func getGaugeValue(t *testing.T, metric prometheus.Gauge) float64 {
	ch := make(chan prometheus.Metric, 1)
	metric.Collect(ch)

	m := <-ch
	pb := &dto.Metric{}

	err := m.Write(pb)
	assert.NoError(t, err)

	return pb.GetGauge().GetValue()
}

func TestWithHistory(t *testing.T) {
	kLines := randomWalkKLines(500, 7)

	type outputs struct {
		boll, superTrend, cci, macdSignal, stochD, atrp []float64
	}

	build := func(source *KLineStream) *outputs {
		boll := BOLL(ClosePrices(source), 20, 2.0)
		superTrend := SuperTrend(source, 10, 3.0)
		cci := CCI(HLC3(source), 20)
		macd := MACD2(ClosePrices(source), 12, 26, 9)
		stoch := Stoch(source, 14, 3)
		atrp := ATRP2(source, 14)

		o := &outputs{}
		source.OnUpdate(func(_ types.KLine) {
			// subscribed last, so all the indicators of this kline are updated
			o.boll = append(o.boll, boll.UpBand.Last(0))
			o.superTrend = append(o.superTrend, superTrend.Last(0))
			o.cci = append(o.cci, cci.Last(0))
			o.macdSignal = append(o.macdSignal, macd.Signal.Last(0))
			o.stochD = append(o.stochD, stoch.D.Last(0))
			o.atrp = append(o.atrp, atrp.Last(0))
		})
		return o
	}

	// the reference: indicators built before any kline is pushed
	live := &KLineStream{}
	expected := build(live)
	live.BackFill(kLines)

	// indicators built on a stream that already has 400 klines, then 100 more klines are pushed
	source := &KLineStream{}
	source.BackFill(kLines[:400])
	actual := WithHistory(source, build)
	source.BackFill(kLines[400:])

	assert.Equal(t, expected.boll, actual.boll, "BOLL")
	assert.Equal(t, expected.superTrend, actual.superTrend, "SuperTrend")
	assert.Equal(t, expected.cci, actual.cci, "CCI")
	assert.Equal(t, expected.macdSignal, actual.macdSignal, "MACD signal")
	assert.Equal(t, expected.stochD, actual.stochD, "Stoch D")
	assert.Equal(t, expected.atrp, actual.atrp, "ATRP")
}
