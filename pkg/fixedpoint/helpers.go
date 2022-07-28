package fixedpoint

func Sum(values []Value) (s Value) {
	s = Zero
	for _, value := range values {
		s = s.Add(value)
	}
	return s
}

func Avg(values []Value) (avg Value) {
	s := Sum(values)
	avg = s.Div(NewFromInt(int64(len(values))))
	return avg
}
