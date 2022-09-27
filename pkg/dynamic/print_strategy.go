package dynamic

import (
	"fmt"
	"io"
	"reflect"
	"unsafe"

	"github.com/c9s/bbgo/pkg/util"
)

// @param s: strategy object
// @param f: io.Writer used for writing the strategy dump
// @param seriesLength: if exist, the first value will be chosen to be the length of data from series to be printed out
//                      default to 1 when not exist or the value is invalid
func ParamDump(s interface{}, f io.Writer, seriesLength ...int) {
	length := 1
	if len(seriesLength) > 0 && seriesLength[0] > 0 {
		length = seriesLength[0]
	}
	val := reflect.ValueOf(s)
	if val.Type().Kind() == util.Pointer {
		val = val.Elem()
	}
	for i := 0; i < val.Type().NumField(); i++ {
		t := val.Type().Field(i)
		if ig := t.Tag.Get("ignore"); ig == "true" {
			continue
		}
		field := val.Field(i)
		if t.IsExported() || t.Anonymous || t.Type.Kind() == reflect.Func || t.Type.Kind() == reflect.Chan {
			continue
		}
		fieldName := t.Name
		typeName := field.Type().String()
		value := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
		isSeries := true
		lastFunc := value.MethodByName("Last")
		isSeries = isSeries && lastFunc.IsValid()
		lengthFunc := value.MethodByName("Length")
		isSeries = isSeries && lengthFunc.IsValid()
		indexFunc := value.MethodByName("Index")
		isSeries = isSeries && indexFunc.IsValid()

		stringFunc := value.MethodByName("String")
		canString := stringFunc.IsValid()

		if isSeries {
			l := int(lengthFunc.Call(nil)[0].Int())
			if l >= length {
				fmt.Fprintf(f, "%s: Series[..., %.4f", fieldName, indexFunc.Call([]reflect.Value{reflect.ValueOf(length - 1)})[0].Float())
				for j := length - 2; j >= 0; j-- {
					fmt.Fprintf(f, ", %.4f", indexFunc.Call([]reflect.Value{reflect.ValueOf(j)})[0].Float())
				}
				fmt.Fprintf(f, "]\n")
			} else if l > 0 {
				fmt.Fprintf(f, "%s: Series[%.4f", fieldName, indexFunc.Call([]reflect.Value{reflect.ValueOf(l - 1)})[0].Float())
				for j := l - 2; j >= 0; j-- {
					fmt.Fprintf(f, ", %.4f", indexFunc.Call([]reflect.Value{reflect.ValueOf(j)})[0].Float())
				}
				fmt.Fprintf(f, "]\n")
			} else {
				fmt.Fprintf(f, "%s: Series[]\n", fieldName)
			}
		} else if canString {
			fmt.Fprintf(f, "%s: %s\n", fieldName, stringFunc.Call(nil)[0].String())
		} else if CanInt(field) {
			fmt.Fprintf(f, "%s: %d\n", fieldName, field.Int())
		} else if field.CanConvert(reflect.TypeOf(float64(0))) {
			fmt.Fprintf(f, "%s: %.4f\n", fieldName, field.Float())
		} else if field.CanInterface() {
			fmt.Fprintf(f, "%s: %v", fieldName, field.Interface())
		} else if field.Type().Kind() == reflect.Map {
			fmt.Fprintf(f, "%s: {", fieldName)
			iter := value.MapRange()
			for iter.Next() {
				k := iter.Key().Interface()
				v := iter.Value().Interface()
				fmt.Fprintf(f, "%v: %v, ", k, v)
			}
			fmt.Fprintf(f, "}\n")
		} else if field.Type().Kind() == reflect.Slice {
			fmt.Fprintf(f, "%s: [", fieldName)
			l := field.Len()
			if l > 0 {
				fmt.Fprintf(f, "%v", field.Index(0))
			}
			for j := 1; j < field.Len(); j++ {
				fmt.Fprintf(f, ", %v", field.Index(j))
			}
			fmt.Fprintf(f, "]\n")
		} else {
			fmt.Fprintf(f, "%s(%s): %s\n", fieldName, typeName, field.String())
		}
	}
}
