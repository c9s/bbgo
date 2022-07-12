package types

import (
	"math"

	"gonum.org/v1/gonum/floats"
)

type Float64Slice []float64

func (s *Float64Slice) Push(v float64) {
	*s = append(*s, v)
}

func (s *Float64Slice) Update(v float64) {
	*s = append(*s, v)
}

func (s *Float64Slice) Pop(i int64) (v float64) {
	v = (*s)[i]
	*s = append((*s)[:i], (*s)[i+1:]...)
	return v
}

func (s Float64Slice) Max() float64 {
	return floats.Max(s)
}

func (s Float64Slice) Min() float64 {
	return floats.Min(s)
}

func (s Float64Slice) Sum() (sum float64) {
	return floats.Sum(s)
}

func (s Float64Slice) Mean() (mean float64) {
	length := len(s)
	if length == 0 {
		panic("zero length slice")
	}
	return s.Sum() / float64(length)
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

func (s Float64Slice) Diff() (values Float64Slice) {
	for i, v := range s {
		if i == 0 {
			values.Push(0)
			continue
		}
		values.Push(v - s[i-1])
	}
	return values
}

func (s Float64Slice) PositiveValuesOrZero() (values Float64Slice) {
	for _, v := range s {
		values.Push(math.Max(v, 0))
	}
	return values
}

func (s Float64Slice) NegativeValuesOrZero() (values Float64Slice) {
	for _, v := range s {
		values.Push(math.Min(v, 0))
	}
	return values
}

func (s Float64Slice) Abs() (values Float64Slice) {
	for _, v := range s {
		values.Push(math.Abs(v))
	}
	return values
}

func (s Float64Slice) MulScalar(x float64) (values Float64Slice) {
	for _, v := range s {
		values.Push(v * x)
	}
	return values
}

func (s Float64Slice) DivScalar(x float64) (values Float64Slice) {
	for _, v := range s {
		values.Push(v / x)
	}
	return values
}

func (s Float64Slice) Mul(other Float64Slice) (values Float64Slice) {
	if len(s) != len(other) {
		panic("slice lengths do not match")
	}

	for i, v := range s {
		values.Push(v * other[i])
	}

	return values
}

func (s Float64Slice) Dot(other Float64Slice) float64 {
	return floats.Dot(s, other)
}

func (s Float64Slice) Normalize() Float64Slice {
	return s.DivScalar(s.Sum())
}

func (a *Float64Slice) Last() float64 {
	length := len(*a)
	if length > 0 {
		return (*a)[length-1]
	}
	return 0.0
}

func (a *Float64Slice) Index(i int) float64 {
	length := len(*a)
	if length-i <= 0 || i < 0 {
		return 0.0
	}
	return (*a)[length-i-1]
}

func (a *Float64Slice) Length() int {
	return len(*a)
}

func (a Float64Slice) Addr() *Float64Slice {
	return &a
}

var _ Series = Float64Slice([]float64{}).Addr()
