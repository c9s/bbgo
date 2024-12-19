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

var matchFirstCapRE = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

var dynamicStrategyConfigMetrics = map[string]*prometheus.GaugeVec{}

func getOrCreateMetric(id, fieldName string) (*prometheus.GaugeVec, error) {
	metricName := id + "_config_" + fieldName
	metric, ok := dynamicStrategyConfigMetrics[metricName]
	if !ok {
		metric = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metricName,
				Help: id + " config value of " + fieldName,
			},
			[]string{"strategy_type", "strategy_id", "symbol"},
		)

		if err := prometheus.Register(metric); err != nil {
			return nil, fmt.Errorf("unable to register metrics on field %+v, error: %+v", fieldName, err)
		}
	}

	return metric, nil
}

func toSnakeCase(input string) string {
	input = matchFirstCapRE.ReplaceAllString(input, "${1}_${2}")
	input = matchAllCap.ReplaceAllString(input, "${1}_${2}")
	return strings.ToLower(input)
}

func castToFloat64(valInf any) (float64, bool) {
	var val float64
	switch tt := valInf.(type) {

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
		return 0.0, false
	}

	return val, true
}

func InitializeConfigMetrics(id, instanceId string, s types.StrategyID) error {
	tv := reflect.TypeOf(s).Elem()
	sv := reflect.Indirect(reflect.ValueOf(s))

	symbolField := sv.FieldByName("Symbol")
	hasSymbolField := symbolField.IsValid()

	for i := 0; i < tv.NumField(); i++ {
		field := tv.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			continue
		}

		tagAttrs := strings.Split(jsonTag, ",")
		if len(tagAttrs) == 0 {
			continue
		}

		fieldName := toSnakeCase(tagAttrs[0])

		val := 0.0
		valInf := sv.Field(i).Interface()
		val, ok := castToFloat64(valInf)
		if !ok {
			continue
		}

		symbol := ""
		if hasSymbolField {
			symbol = symbolField.String()
		}

		metric, err := getOrCreateMetric(id, fieldName)
		if err != nil {
			return err
		}

		metric.With(prometheus.Labels{
			"strategy_type": id,
			"strategy_id":   instanceId,
			"symbol":        symbol,
		}).Set(val)
	}

	return nil
}
