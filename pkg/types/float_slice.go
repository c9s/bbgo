package types

import "math"

type Float64Slice []float64

func (s *Float64Slice) Push(v float64) {
	*s = append(*s, v)
}

func (s *Float64Slice) Pop(i int64) (v float64) {
	v = (*s)[i]
	*s = append((*s)[:i], (*s)[i+1:]...)
	return v
}

func (s Float64Slice) Max() float64 {
	m := -math.MaxFloat64
	for _, v := range s {
		m = math.Max(m, v)
	}
	return m
}

func (s Float64Slice) Min() float64 {
	m := math.MaxFloat64
	for _, v := range s {
		m = math.Min(m, v)
	}
	return m
}

func (s Float64Slice) Sum() (sum float64) {
	for _, v := range s {
		sum += v
	}
	return sum
}

func (s Float64Slice) Mean() (mean float64) {
	return s.Sum() / float64(len(s))
}

func (s Float64Slice) Tail(size int) Float64Slice {
	length := len(s)
	if length <= size {
		win := make(Float64Slice, length)
		copy(win, s)
		return win
	}

	win := make(Float64Slice, size)
	copy(win, s[length-size:])
	return win
}
