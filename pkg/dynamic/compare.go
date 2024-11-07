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

// a (after)
// b (before)
func Compare(a, b interface{}) ([]Diff, error) {
	var diffs []Diff

	ra := reflect.ValueOf(a)
	if ra.Kind() == reflect.Ptr {
		ra = ra.Elem()
	}

	raType := ra.Type()
	raKind := ra.Kind()

	rb := reflect.ValueOf(b)
	if rb.Kind() == reflect.Ptr {
		rb = rb.Elem()
	}

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

	if raKind == reflect.Struct {

	}

	return diffs, nil
}

func compareStruct(a, b reflect.Value) ([]Diff, error) {
	if a.Kind() == reflect.Ptr {
		a = a.Elem()
	}

	if b.Kind() == reflect.Ptr {
		b = b.Elem()
	}

	if a.Kind() != reflect.Struct {
		return nil, fmt.Errorf("value is not a struct")
	}

	if b.Kind() != reflect.Struct {
		return nil, fmt.Errorf("value is not a struct")
	}

	if a.Type() != b.Type() {
		return nil, fmt.Errorf("type is not the same")
	}

	var diffs []Diff

	numFields := a.NumField()
	for i := 0; i < numFields; i++ {
		fieldValueA := a.Field(i)
		fieldValueB := b.Field(i)

		fieldName := a.Type().Field(i).Name

		if isSimpleType(fieldValueA.Kind()) {
			if compareSimpleValue(fieldValueA, fieldValueB) {
				continue
			} else {
				diffs = append(diffs, Diff{
					Field:  fieldName,
					Before: convertToStr(fieldValueB),
					After:  convertToStr(fieldValueA),
				})
			}
		} else if fieldValueA.Kind() == reflect.Struct && fieldValueB.Kind() == reflect.Struct {
			subDiffs, err := compareStruct(fieldValueA, fieldValueB)
			if err != nil {
				return diffs, err
			}

			for _, subDiff := range subDiffs {
				diffs = append(diffs, Diff{
					Field:  fieldName + "." + subDiff.Field,
					Before: subDiff.Before,
					After:  subDiff.After,
				})
			}
		}
	}

	return diffs, nil
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

	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		if a.Uint() == b.Uint() {
			return true
		}
	case reflect.Int, reflect.Int8, reflect.Int32, reflect.Int64:
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

func convertToStr(val reflect.Value) string {
	if val.Type() == reflect.TypeOf(fixedpoint.Zero) {
		fp := val.Interface().(fixedpoint.Value)
		return fp.String()
	}

	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(val.Float(), 'f', -1, 64)

	case reflect.Int, reflect.Int8, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(val.Int(), 10)

	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(val.Uint(), 10)

	case reflect.Bool:
		return strconv.FormatBool(val.Bool())
	default:
		strType := reflect.TypeOf("")
		if val.CanConvert(strType) {
			strVal := val.Convert(strType)
			return strVal.String()
		}

		return "{unable to convert " + val.Kind().String() + "}"
	}
}
