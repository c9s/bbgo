package types

import "github.com/c9s/bbgo/pkg/fixedpoint"

type ValueMap map[string]fixedpoint.Value

func (m ValueMap) Eq(n ValueMap) bool {
	if len(m) != len(n) {
		return false
	}

	for m_k, m_v := range m {
		n_v, ok := n[m_k]
		if !ok {
			return false
		}

		if !m_v.Eq(n_v) {
			return false
		}
	}

	return true
}

func (m ValueMap) Add(n ValueMap) ValueMap {
	if len(m) != len(n) {
		panic("unequal length")
	}

	o := ValueMap{}

	for m_k, m_v := range m {
		n_v, ok := n[m_k]
		if !ok {
			panic("key not found")
		}

		o[m_k] = m_v.Add(n_v)
	}

	return o
}

func (m ValueMap) Sub(n ValueMap) ValueMap {
	if len(m) != len(n) {
		panic("unequal length")
	}

	o := ValueMap{}

	for m_k, m_v := range m {
		n_v, ok := n[m_k]
		if !ok {
			panic("key not found")
		}

		o[m_k] = m_v.Sub(n_v)
	}

	return o
}

func (m ValueMap) Mul(n ValueMap) ValueMap {
	if len(m) != len(n) {
		panic("unequal length")
	}

	o := ValueMap{}

	for m_k, m_v := range m {
		n_v, ok := n[m_k]
		if !ok {
			panic("key not found")
		}

		o[m_k] = m_v.Mul(n_v)
	}

	return o
}

func (m ValueMap) Div(n ValueMap) ValueMap {
	if len(m) != len(n) {
		panic("unequal length")
	}

	o := ValueMap{}

	for m_k, m_v := range m {
		n_v, ok := n[m_k]
		if !ok {
			panic("key not found")
		}

		o[m_k] = m_v.Div(n_v)
	}

	return o
}

func (m ValueMap) AddScalar(x fixedpoint.Value) ValueMap {
	o := ValueMap{}

	for k, v := range m {
		o[k] = v.Add(x)
	}

	return o
}

func (m ValueMap) SubScalar(x fixedpoint.Value) ValueMap {
	o := ValueMap{}

	for k, v := range m {
		o[k] = v.Sub(x)
	}

	return o
}

func (m ValueMap) MulScalar(x fixedpoint.Value) ValueMap {
	o := ValueMap{}

	for k, v := range m {
		o[k] = v.Mul(x)
	}

	return o
}

func (m ValueMap) DivScalar(x fixedpoint.Value) ValueMap {
	o := ValueMap{}

	for k, v := range m {
		o[k] = v.Div(x)
	}

	return o
}

func (m ValueMap) Sum() fixedpoint.Value {
	var sum fixedpoint.Value
	for _, v := range m {
		sum = sum.Add(v)
	}
	return sum
}

func (m ValueMap) Normalize() ValueMap {
	sum := m.Sum()
	if sum.Eq(fixedpoint.Zero) {
		panic("zero sum")
	}

	return m.DivScalar(sum)
}
