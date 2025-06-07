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
	}

	// Create KLineStream instance
	stream := &KLineStream{}

	// Call the function being tested
	stream.metricsKLineUpdater(k)

	// Verify metric values
	labels := prometheus.Labels{
		"exchange":  k.Exchange.String(),
		"symbol":    k.Symbol,
		"interval":  k.Interval.String(),
		"startTime": k.StartTime.Time().Format("2006-01-02 15:04:05"),
		"endTime":   k.EndTime.Time().Format("2006-01-02 15:04:05"),
	}

	// Get metric values
	openValue, err := metricsKLineStreamOpen.GetMetricWith(labels)
	assert.NoError(t, err)
	assert.Equal(t, 50000.0, getGaugeValue(t, openValue))

	closeValue, err := metricsKLineStreamClose.GetMetricWith(labels)
	assert.NoError(t, err)
	assert.Equal(t, 51000.0, getGaugeValue(t, closeValue))

	highValue, err := metricsKLineStreamHigh.GetMetricWith(labels)
	assert.NoError(t, err)
	assert.Equal(t, 52000.0, getGaugeValue(t, highValue))

	lowValue, err := metricsKLineStreamLow.GetMetricWith(labels)
	assert.NoError(t, err)
	assert.Equal(t, 49000.0, getGaugeValue(t, lowValue))
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
