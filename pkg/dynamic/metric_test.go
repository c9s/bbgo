package dynamic

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func TestInitializeConfigMetrics(t *testing.T) {
	type Bar struct {
		Enabled bool `json:"enabled"`
	}
	type Foo struct {
		MinMarginLevel fixedpoint.Value `json:"minMarginLevel"`
		Bar            *Bar             `json:"bar"`

		// this field should be ignored
		ignoredField string

		ignoredFieldInt int
	}

	t.Run("general", func(t *testing.T) {
		metricNames, err := initializeConfigMetricsWithFieldPrefix("test", "test-01", "", "", &Foo{
			MinMarginLevel: Number(1.4),
			Bar: &Bar{
				Enabled: true,
			},
		})

		if assert.NoError(t, err) {
			assert.Len(t, metricNames, 2)
			assert.Equal(t, "test_config_min_margin_level", metricNames[0])
			assert.Equal(t, "test_config_bar_enabled", metricNames[1], "nested struct field as a metric")
		}
	})

	t.Run("nil struct field", func(t *testing.T) {
		metricNames, err := initializeConfigMetricsWithFieldPrefix("test", "test-01", "", "", &Foo{
			MinMarginLevel: Number(1.4),
		})

		if assert.NoError(t, err) {
			assert.Len(t, metricNames, 1)
			assert.Equal(t, "test_config_min_margin_level", metricNames[0])
		}
	})

	t.Run("embedded anonymous struct", func(t *testing.T) {
		type Config struct {
			MinMarginLevel fixedpoint.Value `json:"minMarginLevel"`
			Bar            *Bar             `json:"bar"`
		}

		// Symbol stays on the outer struct while the config knobs live in an
		// embedded anonymous struct; promoted fields must keep flat metric names.
		type Strategy struct {
			Config

			Symbol string `json:"symbol"`
		}

		metricNames, err := initializeConfigMetricsWithFieldPrefix("test2", "test2-01", "", "", &Strategy{
			Symbol: "BTCUSDT",
			Config: Config{
				MinMarginLevel: Number(1.4),
				Bar:            &Bar{Enabled: true},
			},
		})

		if assert.NoError(t, err) {
			assert.Len(t, metricNames, 2)
			assert.Contains(t, metricNames, "test2_config_min_margin_level", "promoted field keeps a flat metric name")
			assert.Contains(t, metricNames, "test2_config_bar_enabled", "nested struct under embedded config")
		}
	})

}
