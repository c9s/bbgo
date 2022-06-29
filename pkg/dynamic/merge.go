package dynamic

import "reflect"

func MergeStructValues(dst, src interface{}) {
	rtA := reflect.TypeOf(dst)
	srcStructType := reflect.TypeOf(src)

	rtA = rtA.Elem()
	srcStructType = srcStructType.Elem()

	for i := 0; i < rtA.NumField(); i++ {
		fieldType := rtA.Field(i)
		fieldName := fieldType.Name
		if fieldSrcType, ok := srcStructType.FieldByName(fieldName); ok {
			if fieldSrcType.Type == fieldType.Type {
				srcValue := reflect.ValueOf(src).Elem().FieldByName(fieldName)
				dstValue := reflect.ValueOf(dst).Elem().FieldByName(fieldName)
				if (fieldType.Type.Kind() == reflect.Ptr && dstValue.IsNil()) || dstValue.IsZero() {
					dstValue.Set(srcValue)
				}
			}
		}
	}
}
