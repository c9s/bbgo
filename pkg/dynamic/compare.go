package dynamic

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

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

	if isSimpleType(ra) {
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
	} else if raKind == reflect.Struct {
		return compareStruct(ra, rb)
	}

	return nil, nil
}

func compareStruct(a, b reflect.Value) ([]Diff, error) {
	a = reflect.Indirect(a)
	b = reflect.Indirect(b)

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

		fieldA := a.Type().Field(i)
		fieldName := fieldA.Name

		if !fieldA.IsExported() {
			continue
		}

		if isSimpleType(fieldValueA) {
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

func isSimpleType(a reflect.Value) bool {
	a = reflect.Indirect(a)
	aInf := a.Interface()

	switch aInf.(type) {
	case time.Time:
		return true

	case fixedpoint.Value:
		return true

	}

	kind := a.Kind()
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
		ainf := a.Interface()
		binf := b.Interface()

		switch aa := ainf.(type) {
		case fixedpoint.Value:
			if bb, ok := binf.(fixedpoint.Value); ok {
				return bb.Compare(aa) == 0
			}
		case time.Time:
			if bb, ok := binf.(time.Time); ok {
				return bb.Compare(aa) == 0
			}
		}

		// other unhandled cases
	}

	return false
}

func convertToStr(val reflect.Value) string {
	val = reflect.Indirect(val)

	if val.Type() == reflect.TypeOf(fixedpoint.Zero) {
		inf := val.Interface()
		switch aa := inf.(type) {
		case fixedpoint.Value:
			return aa.String()
		case time.Time:
			return aa.String()
		}
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
