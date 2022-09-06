package dynamic

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

func DefaultWhiteList() []string {
	return []string{"Window", "RightWindow", "Interval", "Symbol", "Source"}
}

// @param s: strategy object
// @param f: io.Writer used for writing the config dump
// @param style: pretty print table style. Use NewDefaultTableStyle() to get default one.
// @param withColor: whether to print with color
// @param whiteLists: fields to be printed out from embedded struct (1st layer only)
func PrintConfig(s interface{}, f io.Writer, style *table.Style, withColor bool, whiteLists ...string) {
	t := table.NewWriter()
	var write func(io.Writer, string, ...interface{})

	if withColor {
		write = color.New(color.FgHiYellow).FprintfFunc()
	} else {
		write = func(a io.Writer, format string, args ...interface{}) {
			fmt.Fprintf(a, format, args...)
		}
	}
	if style != nil {
		t.SetOutputMirror(f)
		t.SetStyle(*style)
		t.SetColumnConfigs([]table.ColumnConfig{
			{Number: 4, WidthMax: 50, WidthMaxEnforcer: text.WrapText},
		})
		t.AppendHeader(table.Row{"json", "struct field name", "type", "value"})
	}
	write(f, "---- %s Settings ---\n", CallID(s))

	embeddedWhiteSet := map[string]struct{}{}
	for _, whiteList := range whiteLists {
		embeddedWhiteSet[whiteList] = struct{}{}
	}

	redundantSet := map[string]struct{}{}

	var rows []table.Row

	val := reflect.ValueOf(s)

	if val.Type().Kind() == util.Pointer {
		val = val.Elem()
	}
	var values types.JsonArr
	for i := 0; i < val.Type().NumField(); i++ {
		t := val.Type().Field(i)
		if !t.IsExported() {
			continue
		}
		fieldName := t.Name
		switch jsonTag := t.Tag.Get("json"); jsonTag {
		case "-":
		case "":
			// we only fetch fields from the first layer of the embedded struct
			if t.Anonymous {
				var target reflect.Type
				var field reflect.Value
				if t.Type.Kind() == util.Pointer {
					target = t.Type.Elem()
					field = val.Field(i).Elem()
				} else {
					target = t.Type
					field = val.Field(i)
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
						value := field.Field(j).Interface()
						if e, err := json.Marshal(value); err == nil {
							value = string(e)
						}
						values = append(values, types.JsonStruct{Key: fieldName, Json: name, Type: tt.Type.String(), Value: value})
					}
				}
			}
		default:
			name := strings.Split(jsonTag, ",")[0]
			if _, ok := redundantSet[name]; ok {
				continue
			}
			redundantSet[name] = struct{}{}
			value := val.Field(i).Interface()
			if e, err := json.Marshal(value); err == nil {
				value = string(e)
			}
			values = append(values, types.JsonStruct{Key: fieldName, Json: name, Type: t.Type.String(), Value: value})
		}
	}
	sort.Sort(values)
	for _, value := range values {
		if style != nil {
			rows = append(rows, table.Row{value.Json, value.Key, value.Type, value.Value})
		} else {
			write(f, "%s: %v\n", value.Json, value.Value)
		}
	}
	if style != nil {
		t.AppendRows(rows)
		t.Render()
	}
}
