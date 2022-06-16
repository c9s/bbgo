package types

type FloatMap map[string]float64

func (m FloatMap) Sum() float64 {
	sum := 0.0
	for _, v := range m {
		sum += v
	}
	return sum
}

func (m FloatMap) MulScalar(x float64) FloatMap {
	o := FloatMap{}
	for k, v := range m {
		o[k] = v * x
	}

	return o
}
func (m FloatMap) DivScalar(x float64) FloatMap {
	o := FloatMap{}
	for k, v := range m {
		o[k] = v / x
	}

	return o
}

func (m FloatMap) Normalize() FloatMap {
	sum := m.Sum()
	if sum == 0 {
		panic("zero sum")
	}

	o := FloatMap{}
	for k, v := range m {
		o[k] = v / sum
	}

	return o
}
