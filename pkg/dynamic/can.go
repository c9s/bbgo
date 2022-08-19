package dynamic

import "reflect"

// For backward compatibility of reflect.Value.CanInt in go1.17
func CanInt(v reflect.Value) bool {
	k := v.Type().Kind()
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	default:
		return false
	}
}
