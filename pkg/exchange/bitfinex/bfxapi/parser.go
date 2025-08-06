package bfxapi

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/sirupsen/logrus"
)

// isBasicType checks if t is a basic type: int, int8, int16, int32, int64,
// uint, uint8, uint16, uint32, uint64, float32, float64, string or bool.
func isBasicType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String, reflect.Bool:
		return true
	default:
		return false
	}
}

// parseArray uses reflection to decode a slice of json.RawMessage into the struct pointed to by object.
// It maps each element in arr sequentially to each exported field of the struct.
// For basic types (int, float, string, etc.), json.Unmarshal is called directly on the field pointer.
func parseArray(arr []json.RawMessage, object any) error {
	ov := reflect.ValueOf(object)
	if ov.Kind() != reflect.Ptr || ov.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("object must be pointer to struct")
	}
	ov = ov.Elem()
	t := ov.Type()
	n := t.NumField()
	if len(arr) < n {
		return fmt.Errorf("array has insufficient elements: need %d but got %d", n, len(arr))
	}
	for i := 0; i < n; i++ {
		field := ov.Field(i)
		structField := t.Field(i)
		if !field.CanSet() {
			continue // skip unexported fields
		}

		raw := arr[i]
		// For basic types, unmarshal directly.
		if isBasicType(field.Type()) {
			if err := json.Unmarshal(raw, field.Addr().Interface()); err != nil {
				logrus.Errorf("failed to unmarshal basic element %d into field %s: %v", i, structField.Name, err)
				return err
			}
			continue
		}

		// Check if field (or its pointer) implements json.Unmarshaler.
		unmarshalerType := reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
		var target reflect.Value
		if field.Kind() != reflect.Ptr && field.Addr().Type().Implements(unmarshalerType) {
			target = field.Addr()
		} else if field.Type().Implements(unmarshalerType) {
			target = field
		}

		if target.IsValid() {
			if err := json.Unmarshal(raw, target.Interface()); err != nil {
				logrus.Errorf("failed to unmarshal element %d into field %s: %v", i, structField.Name, err)
				return err
			}
			continue
		}
		// Fallback: unmarshal into the field pointer.
		if err := json.Unmarshal(raw, field.Addr().Interface()); err != nil {
			logrus.Errorf("failed to unmarshal element %d into field %s: %v", i, structField.Name, err)
			return err
		}
	}
	return nil
}
