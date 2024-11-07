package dynamic

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Diff struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

func isSimpleType(kind reflect.Kind) bool {
	switch kind {
	case reflect.Bool, reflect.Int, reflect.Int32, reflect.Int64, reflect.Uint64, reflect.String, reflect.Float64:
		return true
	default:
		return false
	}
}

func compareSimpleValue(a, b reflect.Value) bool {
	if a.Kind() != b.Kind() {
		return false
	}

	switch a.Kind() {
	case reflect.Bool:
		if a.Bool() == b.Bool() {
			return true
		}

	case reflect.Int, reflect.Int32, reflect.Int64, reflect.Uint64:
		if a.Int() == b.Int() {
			return true
		}

	case reflect.String:
		if a.String() == b.String() {
			return true
		}

	case reflect.Float64:
		if a.Float() == b.Float() {
			return true
		}

	case reflect.Slice:
		// TODO: compare slice

	default:
		// unhandled case

	}

	return false
}

// a (after)
// b (before)
func Compare(a, b interface{}) ([]Diff, error) {
	var diffs []Diff

	ra := reflect.ValueOf(a)
	raType := ra.Type()
	raKind := ra.Kind()

	rb := reflect.ValueOf(b)
	rbType := rb.Type()
	rbKind := rb.Kind() // bool, int, slice, string, struct

	if raType != rbType {
		return nil, fmt.Errorf("type mismatch: %s != %s", raType, rbType)
	}

	if raKind != rbKind {
		return nil, fmt.Errorf("kind mismatch: %s != %s", raKind, rbKind)
	}

	if isSimpleType(raKind) {
		if compareSimpleValue(ra, rb) {
			// no changes
			return nil, nil
		} else {
			return []Diff{
				{
					Field:  "",
					Before: convertToStr(rb),
					After:  convertToStr(ra),
				},
			}, nil
		}
	}

	return diffs, nil
}

func convertToStr(val reflect.Value) string {
	if val.Type() == reflect.TypeOf(fixedpoint.Zero) {
		fp := val.Interface().(fixedpoint.Value)
		return fp.String()
	}

	switch val.Kind() {
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(val.Float(), 'f', -1, 64)

	case reflect.Int, reflect.Int8, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(val.Int(), 10)

	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(val.Uint(), 10)

	case reflect.Bool:
		if val.Bool() {
			return "true"
		} else {
			return "false"
		}
	default:
		strType := reflect.TypeOf("")
		if val.CanConvert(strType) {
			strVal := val.Convert(strType)
			return strVal.String()
		}

		return "{unable to convert " + val.Kind().String() + "}"
	}
}
