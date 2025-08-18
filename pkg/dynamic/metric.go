package dynamic

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var matchFirstCapRE = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

var dynamicStrategyConfigMetrics = map[string]*prometheus.GaugeVec{}
var dynamicStrategyConfigMetricsMutex sync.Mutex

func getOrCreateMetric(id, fieldName string) (*prometheus.GaugeVec, string, error) {
	metricName := id + "_config_" + fieldName

	dynamicStrategyConfigMetricsMutex.Lock()
	metric, ok := dynamicStrategyConfigMetrics[metricName]
	defer dynamicStrategyConfigMetricsMutex.Unlock()

	if !ok {
		metric = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metricName,
				Help: id + " config value of " + fieldName,
			},
			[]string{"strategy_type", "strategy_id", "symbol"},
		)

		if err := prometheus.Register(metric); err != nil {
			return nil, "", fmt.Errorf("unable to register metrics on field %+v, error: %+v", fieldName, err)
		}

		dynamicStrategyConfigMetrics[metricName] = metric
	}

	return metric, metricName, nil
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

func InitializeConfigMetrics(id, instanceId string, st any) error {
	_, err := initializeConfigMetricsWithFieldPrefix(id, instanceId, "", st)
	return err
}

func initializeConfigMetricsWithFieldPrefix(id, instanceId, fieldPrefix string, st any) ([]string, error) {
	var metricNames []string
	tv := reflect.TypeOf(st).Elem()

	vv := reflect.ValueOf(st)
	if vv.IsNil() {
		return nil, nil
	}

	sv := reflect.Indirect(vv)

	symbolField := sv.FieldByName("Symbol")
	hasSymbolField := symbolField.IsValid()

	for i := 0; i < tv.NumField(); i++ {
		field := tv.Field(i)
		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			continue
		}

		tagAttrs := strings.Split(jsonTag, ",")
		if len(tagAttrs) == 0 {
			continue
		}

		fieldName := fieldPrefix + toSnakeCase(tagAttrs[0])
		if field.Type.Kind() == reflect.Pointer && field.Type.Elem().Kind() == reflect.Struct {
			subMetricNames, err := initializeConfigMetricsWithFieldPrefix(id, instanceId, fieldName+"_", sv.Field(i).Interface())
			if err != nil {
				return nil, err
			}

			metricNames = append(metricNames, subMetricNames...)
			continue
		}

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

		metric, metricName, err := getOrCreateMetric(id, fieldName)
		if err != nil {
			return nil, err
		}

		metric.With(prometheus.Labels{
			"strategy_type": id,
			"strategy_id":   instanceId,
			"symbol":        symbol,
		}).Set(val)

		metricNames = append(metricNames, metricName)
	}

	return metricNames, nil
}
