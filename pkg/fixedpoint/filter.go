package fixedpoint

type Tester func(value Value) bool

func PositiveTester(value Value) bool {
	return value.Sign() > 0
}

func NegativeTester(value Value) bool {
	return value.Sign() < 0
}

func Filter(values []Value, f Tester) (slice []Value) {
	for _, v := range values {
		if f(v) {
			slice = append(slice, v)
		}
	}
	return slice
}
