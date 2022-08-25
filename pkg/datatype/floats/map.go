package floats

type Map map[string]float64

func (m Map) Sum() float64 {
	sum := 0.0
	for _, v := range m {
		sum += v
	}
	return sum
}

func (m Map) MulScalar(x float64) Map {
	o := Map{}
	for k, v := range m {
		o[k] = v * x
	}

	return o
}
func (m Map) DivScalar(x float64) Map {
	o := Map{}
	for k, v := range m {
		o[k] = v / x
	}

	return o
}

func (m Map) Normalize() Map {
	sum := m.Sum()
	if sum == 0 {
		panic("zero sum")
	}

	o := Map{}
	for k, v := range m {
		o[k] = v / sum
	}

	return o
}
