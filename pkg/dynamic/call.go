package dynamic

import (
	"errors"
	"reflect"
)

// CallStructFieldsMethod iterates field from the given struct object
// check if the field object implements the interface, if it's implemented, then we call a specific method
func CallStructFieldsMethod(m interface{}, method string, args ...interface{}) error {
	rv := reflect.ValueOf(m)
	rt := reflect.TypeOf(m)

	if rt.Kind() != reflect.Ptr {
		return errors.New("the given object needs to be a pointer")
	}

	rv = rv.Elem()
	rt = rt.Elem()

	if rt.Kind() != reflect.Struct {
		return errors.New("the given object needs to be struct")
	}

	argValues := ToReflectValues(args...)
	for i := 0; i < rt.NumField(); i++ {
		fieldType := rt.Field(i)
		fieldValue := rv.Field(i)

		// skip non-exported fields
		if !fieldType.IsExported() {
			continue
		}

		if fieldType.Type.Kind() == reflect.Ptr && fieldValue.IsNil() {
			continue
		}

		methodType, ok := fieldType.Type.MethodByName(method)
		if !ok {
			continue
		}

		if len(argValues) < methodType.Type.NumIn() {
			// return fmt.Errorf("method %v require %d args, %d given", methodType, methodType.Type.NumIn(), len(argValues))
		}

		refMethod := fieldValue.MethodByName(method)
		refMethod.Call(argValues)
	}

	return nil
}

// CallMatch calls the function with the matched argument automatically
// you can define multiple parameter factory function to inject the return value as the function argument.
// e.g.,
//     CallMatch(targetFunction, 1, 10, true, func() *ParamType { .... })
//
func CallMatch(f interface{}, objects ...interface{}) ([]reflect.Value, error) {
	fv := reflect.ValueOf(f)
	ft := reflect.TypeOf(f)

	var startIndex = 0
	var fArgs []reflect.Value

	var factoryParams = findFactoryParams(objects...)

nextDynamicInputArg:
	for i := 0; i < ft.NumIn(); i++ {
		at := ft.In(i)

		// uat == underlying argument type
		uat := at
		if at.Kind() == reflect.Ptr {
			uat = at.Elem()
		}

		for oi := startIndex; oi < len(objects); oi++ {
			var obj = objects[oi]
			var objT = reflect.TypeOf(obj)
			if objT == at {
				fArgs = append(fArgs, reflect.ValueOf(obj))
				startIndex = oi + 1
				continue nextDynamicInputArg
			}

			// get the kind of argument
			switch k := uat.Kind(); k {

			case reflect.Interface:
				if objT.Implements(at) {
					fArgs = append(fArgs, reflect.ValueOf(obj))
					startIndex = oi + 1
					continue nextDynamicInputArg
				}
			}
		}

		// factory param can be reused
		for _, fp := range factoryParams {
			fpt := fp.Type()
			outType := fpt.Out(0)
			if outType == at {
				fOut := fp.Call(nil)
				fArgs = append(fArgs, fOut[0])
				continue nextDynamicInputArg
			}
		}

		fArgs = append(fArgs, reflect.Zero(at))
	}

	out := fv.Call(fArgs)
	if ft.NumOut() == 0 {
		return out, nil
	}

	// try to get the error object from the return value (if any)
	var err error
	for i := 0; i < ft.NumOut(); i++ {
		outType := ft.Out(i)
		switch outType.Kind() {
		case reflect.Interface:
			o := out[i].Interface()
			switch ov := o.(type) {
			case error:
				err = ov

			}

		}
	}
	return out, err
}

func findFactoryParams(objs ...interface{}) (fs []reflect.Value) {
	for i := range objs {
		obj := objs[i]

		objT := reflect.TypeOf(obj)

		if objT.Kind() != reflect.Func {
			continue
		}

		if objT.NumOut() == 0 || objT.NumIn() > 0 {
			continue
		}

		fs = append(fs, reflect.ValueOf(obj))
	}

	return fs
}
