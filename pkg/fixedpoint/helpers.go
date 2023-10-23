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

// maxDiff is the maximum deviation between a and b to consider them approximately equal
func ApproxEqual(a, b Value, maxDiff float64) bool {
	// Calculate the absolute difference
	diff := Abs(a.Sub(b))

	// Define the small multiple
	smallMultiple := a.Mul(NewFromFloat(maxDiff))

	// Compare the absolute difference to the small multiple
	return diff <= smallMultiple
}
