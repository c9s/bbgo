package util

import (
	"reflect"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// FilterSimpleArgs filters out the simple type arguments
// int, string, bool, and []byte
func FilterSimpleArgs(args []interface{}) (simpleArgs []interface{}) {
	for _, arg := range args {
		switch arg.(type) {
		case int, int64, int32, uint64, uint32, string, []string, []byte, float64, []float64, float32, fixedpoint.Value, time.Time:
			simpleArgs = append(simpleArgs, arg)
		default:
			rt := reflect.TypeOf(arg)
			if rt.Kind() == reflect.Ptr {
				rt = rt.Elem()
			}

			switch rt.Kind() {
			case reflect.Float64, reflect.Float32, reflect.String, reflect.Int, reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64, reflect.Bool:
				simpleArgs = append(simpleArgs, arg)
			}
		}
	}

	return simpleArgs
}
