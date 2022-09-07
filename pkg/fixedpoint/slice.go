package fixedpoint

type Slice []Value

func (s Slice) Reduce(init Value, reducer Reducer) Value {
	return Reduce(s, init, reducer)
}
