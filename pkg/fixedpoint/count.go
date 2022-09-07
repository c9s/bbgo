package fixedpoint

type Counter func(a Value) bool

func Count(values []Value, counter Counter) int {
	var c = 0
	for _, value := range values {
		if counter(value) {
			c++
		}
	}
	return c
}
