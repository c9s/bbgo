package fixedpoint

type Reducer func(prev, curr Value) Value

func Reduce(values []Value, init Value, reducer Reducer) Value {
	if len(values) == 0 {
		return init
	}

	r := reducer(init, values[0])
	for i := 1; i < len(values); i++ {
		r = reducer(r, values[i])
	}

	return r
}
