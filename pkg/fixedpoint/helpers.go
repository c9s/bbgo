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

// ApproxEqual reports whether a and b are within a relative tolerance of each
// other. maxDiff is the tolerance expressed as a fraction of |a|; for example
// 0.01 compares a and b with a 1% tolerance.
func ApproxEqual(a, b Value, maxDiff float64) bool {
	diff := a.Sub(b).Abs()
	tolerance := a.Abs().Mul(NewFromFloat(maxDiff))
	return diff.Compare(tolerance) <= 0
}
