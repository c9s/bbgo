package dynamic

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var dynamicStrategyConfigMetrics = map[string]any{}

func InitializeConfigMetrics(id, instanceId string, s types.StrategyID) error {
	matchFirstCapRE := regexp.MustCompile("(.)([A-Z][a-z]+)")

	tv := reflect.TypeOf(s).Elem()
	sv := reflect.Indirect(reflect.ValueOf(s))

	symbolField := sv.FieldByName("Symbol")
	hasSymbolField := symbolField.IsValid()

nextStructField:
	for i := 0; i < tv.NumField(); i++ {
		field := tv.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			continue nextStructField
		}

		tagAttrs := strings.Split(jsonTag, ",")
		if len(tagAttrs) == 0 {
			continue nextStructField
		}

		fieldName := tagAttrs[0]
		fieldName = strings.ToLower(matchFirstCapRE.ReplaceAllString(fieldName, "${1}_${2}"))

		isStr := false

		val := 0.0
		valInf := sv.Field(i).Interface()
		switch tt := valInf.(type) {
		case string:
			isStr = true

		case fixedpoint.Value:
			val = tt.Float64()
		case *fixedpoint.Value:
			if tt != nil {
				val = tt.Float64()
			}
		case float64:
			val = tt
		case int:
			val = float64(tt)
		case int32:
			val = float64(tt)
		case int64:
			val = float64(tt)
		case bool:
			if tt {
				val = 1.0
			} else {
				val = 0.0
			}
		default:
			continue nextStructField
		}

		if isStr {
			continue nextStructField
		}

		symbol := ""
		if hasSymbolField {
			symbol = symbolField.String()
		}

		metricName := id + "_config_" + fieldName
		anyMetric, ok := dynamicStrategyConfigMetrics[metricName]
		if !ok {
			gaugeMetric := prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: metricName,
					Help: id + " config value of " + field.Name,
				},
				[]string{"strategy_type", "strategy_id", "symbol"},
			)
			if err := prometheus.Register(gaugeMetric); err != nil {
				return fmt.Errorf("unable to register metrics on field %+v, error: %+v", field.Name, err)
			}

			anyMetric = gaugeMetric
			dynamicStrategyConfigMetrics[metricName] = anyMetric
		}

		if anyMetric != nil {
			switch metric := anyMetric.(type) {
			case *prometheus.GaugeVec:
				metric.With(prometheus.Labels{
					"strategy_type": id,
					"strategy_id":   instanceId,
					"symbol":        symbol,
				}).Set(val)
			}
		}
	}

	return nil
}
