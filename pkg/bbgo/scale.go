package bbgo

import (
	"fmt"
	"math"
)

// y := ab^x
// shift xs[0] to 0 (x - h)
// a = y1
//
// y := ab^(x-h)
// y2/a = b^(x2-h)
// y2/y1 = b^(x2-h)
//
// also posted at https://play.golang.org/p/JlWlwZjoebE
type ExpScale struct {
	Domain [2]float64 `json:"domain"`
	Range  [2]float64 `json:"range"`

	a float64
	b float64
	h float64
}

func (s *ExpScale) Solve() error {
	s.h = s.Domain[0]
	s.a = s.Range[0]
	s.b = math.Pow(s.Range[1]/s.Range[0], 1/(s.Domain[1]-s.h))
	return nil
}

func (s *ExpScale) String() string {
	return s.Formula()
}

func (s *ExpScale) Formula() string {
	return fmt.Sprintf("f(x) = %f * %f ^ (x - %f)", s.a, s.b, s.h)
}

func (s *ExpScale) FormulaOf(x float64) string {
	return fmt.Sprintf("f(%f) = %f * %f ^ (%f - %f)", x, s.a, s.b, x, s.h)
}

func (s *ExpScale) Call(x float64) (y float64) {
	y = s.a * math.Pow(s.b, x-s.h)
	return y
}

type LogScale struct {
	Domain [2]float64 `json:"domain"`
	Range  [2]float64 `json:"range"`

	h float64
	s float64
	a float64
}

func (s *LogScale) Call(x float64) (y float64) {
	// y = a * log(x - h) + s
	y = s.a*math.Log(x-s.h) + s.s
	return y
}

func (s *LogScale) String() string {
	return s.Formula()
}

func (s *LogScale) Formula() string {
	return fmt.Sprintf("f(x) = %f * log(x - %f) + %f", s.a, s.h, s.s)
}

func (s *LogScale) FormulaOf(x float64) string {
	return fmt.Sprintf("f(%f) = %f * log(%f - %f) + %f", x, s.a, x, s.h, s.s)
}

func (s *LogScale) Solve() error {
	// f(x) = a * log2(x - h) + s
	//
	// log2(1) = 0
	//
	// h = x1 - 1
	// s = y1
	//
	// y2 = a * log(x2 - h) + s
	// y2 = a * log(x2 - h) + y1
	// y2 - y1 = a * log(x2 - h)
	// a = (y2 - y1) / log(x2 - h)
	s.h = s.Domain[0] - 1
	s.s = s.Range[0]
	s.a = (s.Range[1] - s.Range[0]) / math.Log(s.Domain[1]-s.h)
	return nil
}

type QuadraticScale struct {
	Domain [3]float64 `json:"domain"`
	Range  [3]float64 `json:"range"`

	a, b, c float64
}

func (s *QuadraticScale) Solve() error {
	xs := s.Domain
	ys := s.Range
	s.a = ((ys[1]-ys[0])*(xs[0]-xs[2]) + (ys[2]-ys[0])*(xs[1]-xs[0])) /
		((xs[0]-xs[2])*(math.Pow(xs[1], 2)-math.Pow(xs[0], 2)) + (xs[1]-xs[0])*(math.Pow(xs[2], 2)-math.Pow(xs[0], 2)))

	s.b = ((ys[1] - ys[0]) - s.a*(math.Pow(xs[1], 2)-math.Pow(xs[0], 2))) / (xs[1] - xs[0])
	s.c = ys[1] - s.a*math.Pow(xs[1], 2) - s.b*xs[1]
	return nil
}

func (s *QuadraticScale) Call(x float64) (y float64) {
	// y = a * log(x - h) + s
	y = s.a * math.Pow(x, 2) + s.b * x + s.c
	return y
}

func (s *QuadraticScale) String() string {
	return s.Formula()
}

func (s *QuadraticScale) Formula() string {
	return fmt.Sprintf("f(x) = %f * x ^ 2 + %f * x + %f", s.a, s.b, s.c)
}

func (s *QuadraticScale) FormulaOf(x float64) string {
	return fmt.Sprintf("f(%f) = %f * %f ^ 2 + %f * %f + %f", x, s.a, x, s.b, x, s.c)
}
