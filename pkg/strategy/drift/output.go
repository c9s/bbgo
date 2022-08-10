package drift

import (
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"unsafe"

	"github.com/c9s/bbgo/pkg/util"
	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

type jsonStruct struct {
	key   string
	json  string
	tp    string
	value interface{}
}
type jsonArr []jsonStruct

func (a jsonArr) Len() int           { return len(a) }
func (a jsonArr) Less(i, j int) bool { return a[i].key < a[j].key }
func (a jsonArr) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func (s *Strategy) ParamDump(f io.Writer, seriesLength ...int) {
	length := 1
	if len(seriesLength) > 0 && seriesLength[0] > 0 {
		length = seriesLength[0]
	}
	val := reflect.ValueOf(s).Elem()
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
		} else if field.CanInt() {
			fmt.Fprintf(f, "%s: %d\n", fieldName, field.Int())
		} else if field.CanFloat() {
			fmt.Fprintf(f, "%s: %.4f\n", fieldName, field.Float())
		} else if field.CanInterface() {
			fmt.Fprintf(f, "%s: %v", fieldName, field.Interface())
		} else if field.Type().Kind() == reflect.Map {
			fmt.Fprintf(f, "%s: {", fieldName)
			iter := field.MapRange()
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

func (s *Strategy) Print(f io.Writer, pretty bool, withColor ...bool) {
	//b, _ := json.MarshalIndent(s.ExitMethods, "  ", "  ")

	t := table.NewWriter()
	style := table.Style{
		Name:    "StyleRounded",
		Box:     table.StyleBoxRounded,
		Color:   table.ColorOptionsDefault,
		Format:  table.FormatOptionsDefault,
		HTML:    table.DefaultHTMLOptions,
		Options: table.OptionsDefault,
		Title:   table.TitleOptionsDefault,
	}
	var hiyellow func(io.Writer, string, ...interface{})
	if len(withColor) > 0 && withColor[0] {
		if pretty {
			style.Color = table.ColorOptionsYellowWhiteOnBlack
			style.Color.Row = text.Colors{text.FgHiYellow, text.BgHiBlack}
			style.Color.RowAlternate = text.Colors{text.FgYellow, text.BgBlack}
		}
		hiyellow = color.New(color.FgHiYellow).FprintfFunc()
	} else {
		hiyellow = func(a io.Writer, format string, args ...interface{}) {
			fmt.Fprintf(a, format, args...)
		}
	}
	if pretty {
		t.SetOutputMirror(f)
		t.SetStyle(style)
		t.AppendHeader(table.Row{"json", "struct field name", "type", "value"})
	}
	hiyellow(f, "------ %s Settings ------\n", s.InstanceID())

	embeddedWhiteSet := map[string]struct{}{"Window": {}, "Interval": {}, "Symbol": {}}
	redundantSet := map[string]struct{}{}
	var rows []table.Row
	val := reflect.ValueOf(*s)
	var values jsonArr
	for i := 0; i < val.Type().NumField(); i++ {
		t := val.Type().Field(i)
		if !t.IsExported() {
			continue
		}
		fieldName := t.Name
		switch jsonTag := t.Tag.Get("json"); jsonTag {
		case "-":
		case "":
			if t.Anonymous {
				var target reflect.Type
				if t.Type.Kind() == util.Pointer {
					target = t.Type.Elem()
				} else {
					target = t.Type
				}
				for j := 0; j < target.NumField(); j++ {
					tt := target.Field(j)
					if !tt.IsExported() {
						continue
					}
					fieldName := tt.Name
					if _, ok := embeddedWhiteSet[fieldName]; !ok {
						continue
					}

					if jtag := tt.Tag.Get("json"); jtag != "" && jtag != "-" {
						name := strings.Split(jtag, ",")[0]
						if _, ok := redundantSet[name]; ok {
							continue
						}
						redundantSet[name] = struct{}{}
						var value interface{}
						if t.Type.Kind() == util.Pointer {
							value = val.Field(i).Elem().Field(j).Interface()
						} else {
							value = val.Field(i).Field(j).Interface()
						}
						values = append(values, jsonStruct{key: fieldName, json: name, tp: tt.Type.String(), value: value})
					}
				}
			}
		default:
			name := strings.Split(jsonTag, ",")[0]
			if _, ok := redundantSet[name]; ok {
				continue
			}
			redundantSet[name] = struct{}{}
			values = append(values, jsonStruct{key: fieldName, json: name, tp: t.Type.String(), value: val.Field(i).Interface()})
		}
	}
	sort.Sort(values)
	for _, value := range values {
		if pretty {
			rows = append(rows, table.Row{value.json, value.key, value.tp, value.value})
		} else {
			hiyellow(f, "%s: %v\n", value.json, value.value)
		}
	}
	if pretty {
		rows = append(rows, table.Row{"takeProfitFactor(last)", "takeProfitFactor", "float64", s.takeProfitFactor.Last()})
		t.AppendRows(rows)
		t.Render()
	} else {
		hiyellow(f, "takeProfitFactor(last): %f\n", s.takeProfitFactor.Last())
	}
}
