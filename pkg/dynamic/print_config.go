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
// @param whiteLists: deprecated no-op. All json-tagged fields of embedded
// anonymous structs are now printed as if declared on the parent struct.
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

	redundantSet := map[string]struct{}{}

	var rows []table.Row

	val := reflect.ValueOf(s)

	if val.Type().Kind() == util.Pointer {
		val = val.Elem()
	}

	var values types.JsonArr

	// appendField renders a single struct field into the values slice, honoring
	// the json / ignore tags and de-duplicating by json name.
	appendField := func(sf reflect.StructField, fv reflect.Value) {
		if !sf.IsExported() {
			return
		}

		jsonTag := sf.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			return
		}

		if ig := sf.Tag.Get("ignore"); ig == "true" {
			return
		}

		name := strings.Split(jsonTag, ",")[0]
		if _, ok := redundantSet[name]; ok {
			return
		}

		redundantSet[name] = struct{}{}

		value := fv.Interface()
		if e, err := json.Marshal(value); err == nil {
			value = string(e)
		}

		values = append(values, types.JsonStruct{Key: sf.Name, Json: name, Type: sf.Type.String(), Value: value})
	}

	for i := 0; i < val.Type().NumField(); i++ {
		t := val.Type().Field(i)
		if !t.IsExported() {
			continue
		}

		// Expand embedded (anonymous) structs so that their promoted json fields
		// are printed as if they were declared directly on the parent struct.
		if t.Anonymous && t.Tag.Get("json") == "" {
			var target reflect.Type
			var field reflect.Value
			if t.Type.Kind() == util.Pointer {
				if val.Field(i).IsNil() {
					continue
				}

				target = t.Type.Elem()
				field = val.Field(i).Elem()
			} else {
				target = t.Type
				field = val.Field(i)
			}

			for j := 0; j < target.NumField(); j++ {
				appendField(target.Field(j), field.Field(j))
			}

			continue
		}

		appendField(t, val.Field(i))
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
