package floats

func (s Slice) Pivot(left, right int, f func(a, pivot float64) bool) (float64, bool) {
	return FindPivot(s, left, right, f)
}

func FindPivot(values Slice, left, right int, f func(a, pivot float64) bool) (float64, bool) {
	length := len(values)

	if right == 0 {
		right = left
	}

	if length == 0 || length < left+right+1 {
		return 0.0, false
	}

	end := length - 1
	index := end - right
	val := values[index]

	for i := index - left; i <= index+right; i++ {
		if i == index {
			continue
		}

		// return if we found lower value
		if !f(values[i], val) {
			return 0.0, false
		}
	}

	return val, true
}
