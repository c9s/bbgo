package floats

import (
	"math"

	"gonum.org/v1/gonum/floats"
)

type Slice []float64

func New(a ...float64) Slice {
	return Slice(a)
}

func (s *Slice) Push(v float64) {
	*s = append(*s, v)
}

func (s *Slice) Append(vs ...float64) {
	*s = append(*s, vs...)
}

// Update equals to Push()
// which push an element into the slice
func (s *Slice) Update(v float64) {
	*s = append(*s, v)
}

func (s *Slice) Pop(i int64) (v float64) {
	v = (*s)[i]
	*s = append((*s)[:i], (*s)[i+1:]...)
	return v
}

func (s Slice) Max() float64 {
	return floats.Max(s)
}

func (s Slice) Min() float64 {
	return floats.Min(s)
}

func (s Slice) Sub(b Slice) (c Slice) {
	if len(s) != len(b) {
		return c
	}

	c = make(Slice, len(s))
	for i := 0; i < len(s); i++ {
		ai := s[i]
		bi := b[i]
		ci := ai - bi
		c[i] = ci
	}

	return c
}

func (s Slice) Add(b Slice) (c Slice) {
	if len(s) != len(b) {
		return c
	}

	c = make(Slice, len(s))
	for i := 0; i < len(s); i++ {
		ai := s[i]
		bi := b[i]
		ci := ai + bi
		c[i] = ci
	}

	return c
}

func (s Slice) Sum() (sum float64) {
	for _, v := range s {
		sum += v
	}
	return sum
}

func (s Slice) Mean() (mean float64) {
	length := len(s)
	if length == 0 {
		panic("zero length slice")
	}
	return s.Sum() / float64(length)
}

func (s Slice) Tail(size int) Slice {
	length := len(s)
	if length <= size {
		win := make(Slice, length)
		copy(win, s)
		return win
	}

	win := make(Slice, size)
	copy(win, s[length-size:])
	return win
}

func (s Slice) Average() float64 {
	if len(s) == 0 {
		return 0.0
	}

	total := 0.0
	for _, value := range s {
		total += value
	}
	return total / float64(len(s))
}

func (s Slice) Diff() (values Slice) {
	for i, v := range s {
		if i == 0 {
			values.Push(0)
			continue
		}
		values.Push(v - s[i-1])
	}
	return values
}

func (s Slice) PositiveValuesOrZero() (values Slice) {
	for _, v := range s {
		values.Push(math.Max(v, 0))
	}
	return values
}

func (s Slice) NegativeValuesOrZero() (values Slice) {
	for _, v := range s {
		values.Push(math.Min(v, 0))
	}
	return values
}

func (s Slice) Abs() (values Slice) {
	for _, v := range s {
		values.Push(math.Abs(v))
	}
	return values
}

func (s Slice) MulScalar(x float64) (values Slice) {
	for _, v := range s {
		values.Push(v * x)
	}
	return values
}

func (s Slice) DivScalar(x float64) (values Slice) {
	for _, v := range s {
		values.Push(v / x)
	}
	return values
}

func (s Slice) Mul(other Slice) (values Slice) {
	if len(s) != len(other) {
		panic("slice lengths do not match")
	}

	for i, v := range s {
		values.Push(v * other[i])
	}

	return values
}

func (s Slice) Dot(other Slice) float64 {
	return floats.Dot(s, other)
}

func (s Slice) Normalize() Slice {
	return s.DivScalar(s.Sum())
}

func (s Slice) Addr() *Slice {
	return &s
}

// Last, Index, Length implements the types.Series interface
func (s Slice) Last(i int) float64 {
	length := len(s)
	if i < 0 || length-1-i < 0 {
		return 0.0
	}
	return s[length-1-i]
}

func (s Slice) Truncate(size int) Slice {
	if size < 0 || len(s) <= size {
		return s
	}

	return s[len(s)-size:]
}

// Index fetches the element from the end of the slice
// WARNING: it does not start from 0!!!
func (s Slice) Index(i int) float64 {
	return s.Last(i)
}

func (s Slice) Length() int {
	return len(s)
}

func (s Slice) LSM() float64 {
	return LSM(s)
}

// LSM is the least squares method for linear regression
func LSM(values Slice) float64 {
	var sumX, sumY, sumXSqr, sumXY = .0, .0, .0, .0

	end := len(values) - 1
	for i := end; i >= 0; i-- {
		val := values[i]
		per := float64(end - i + 1)
		sumX += per
		sumY += val
		sumXSqr += per * per
		sumXY += val * per
	}

	length := float64(len(values))
	slope := (length*sumXY - sumX*sumY) / (length*sumXSqr - sumX*sumX)

	average := sumY / length
	tail := average - slope*sumX/length + slope
	head := tail + slope*(length-1)
	slope2 := (tail - head) / (length - 1)
	return slope2
}
