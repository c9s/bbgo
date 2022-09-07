package fixedpoint

type Reducer func(prev, curr Value) Value

func SumReducer(prev, curr Value) Value {
	return prev.Add(curr)
}

func Reduce(values []Value, reducer Reducer, a ...Value) Value {
	init := Zero
	if len(a) > 0 {
		init = a[0]
	}

	if len(values) == 0 {
		return init
	}

	r := reducer(init, values[0])
	for i := 1; i < len(values); i++ {
		r = reducer(r, values[i])
	}

	return r
}
