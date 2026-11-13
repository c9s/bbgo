package binancecsv

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/archive"
	"github.com/c9s/bbgo/pkg/types"
)

var metricsColumns = []string{
	"create_time", "symbol",
	"sum_open_interest", "sum_open_interest_value",
	"count_toptrader_long_short_ratio", "sum_toptrader_long_short_ratio",
	"count_long_short_ratio", "sum_taker_long_short_vol_ratio",
}

// metricColumnKeys maps an archive column to the key it takes in
// marketdata.Metrics.Values.
var metricColumnKeys = []struct {
	column   string
	position int
	key      string
}{
	{"sum_open_interest", 2, marketdata.MetricSumOpenInterest},
	{"sum_open_interest_value", 3, marketdata.MetricSumOpenInterestValue},
	{"count_toptrader_long_short_ratio", 4, marketdata.MetricCountTopTraderLongShortRatio},
	{"sum_toptrader_long_short_ratio", 5, marketdata.MetricSumTopTraderLongShortRatio},
	{"count_long_short_ratio", 6, marketdata.MetricCountLongShortRatio},
	{"sum_taker_long_short_vol_ratio", 7, marketdata.MetricSumTakerLongShortVolRatio},
}

type metricsDecoder struct{ cfg Config }

func newMetricsDecoder(cfg Config) RecordDecoder { return &metricsDecoder{cfg: cfg} }

func (d *metricsDecoder) Columns() []string { return metricsColumns }

func (d *metricsDecoder) Decode(
	dst []marketdata.Event, record []string, meta RecordMeta,
) ([]marketdata.Event, error) {
	if len(record) < 8 {
		return dst, fmt.Errorf("binancecsv: metrics record has %d fields, want at least 8", len(record))
	}

	ts, err := archive.ParseDateTime(record[meta.column("create_time", 0)])
	if err != nil {
		return dst, fmt.Errorf("binancecsv: bad create_time: %w", err)
	}

	values := make(map[string]fixedpoint.Value, len(metricColumnKeys))
	for _, m := range metricColumnKeys {
		i := meta.column(m.column, m.position)
		if i >= len(record) {
			continue
		}
		// Binance leaves individual ratio cells empty on low-activity symbols,
		// so a missing value is skipped rather than failing the record.
		v, err := fixedpoint.NewFromString(record[i])
		if err != nil {
			continue
		}
		values[m.key] = v
	}

	return append(dst, marketdata.Event{
		Type:     marketdata.EventTypeMetrics,
		Exchange: meta.Exchange,
		Symbol:   meta.Symbol,
		Key: marketdata.OrderKey{
			TimeNs: ts.UnixNano(),
			Rank:   marketdata.RankMetrics,
		},
		Metrics: &marketdata.Metrics{
			Symbol: meta.Symbol,
			Time:   types.Time(ts),
			Values: values,
		},
	}), nil
}
