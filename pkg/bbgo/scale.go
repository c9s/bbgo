package bbgo

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/pkg/errors"
)

type Scale interface {
	Solve() error
	Formula() string
	FormulaOf(x float64) string
	Call(x float64) (y float64)
	Sum(step float64) float64
}

func init() {
	_ = Scale(&ExponentialScale{})
	_ = Scale(&LogarithmicScale{})
	_ = Scale(&LinearScale{})
	_ = Scale(&QuadraticScale{})
}

// f(x) := ab^x
// y := ab^x
// shift xs[0] to 0 (x - h)
// a = y1
//
// y := ab^(x-h)
// y2/a = b^(x2-h)
// y2/y1 = b^(x2-h)
//
// also posted at https://play.golang.org/p/JlWlwZjoebE
type ExponentialScale struct {
	Domain [2]float64 `json:"domain"`
	Range  [2]float64 `json:"range"`

	a float64
	b float64
	h float64
	s float64
}

func (s *ExponentialScale) Solve() error {
	if s.Domain[0] > s.Domain[1] {
		return errors.New("domain[0] can not greater than domain[1]")
	}

	if s.Range[0] == 0 {
		return errors.New("for ExponentialScale, range can not start from 0")
	}

	s.h = s.Domain[0]
	s.a = s.Range[0]
	s.b = math.Pow(s.Range[1]/s.Range[0], 1/(s.Domain[1]-s.h))
	s.s = s.Domain[1] - s.h
	return nil
}

func (s *ExponentialScale) Sum(step float64) float64 {
	sum := 0.0
	for x := s.Domain[0]; x <= s.Domain[1]; x += step {
		sum += s.Call(x)
	}
	return sum
}

func (s *ExponentialScale) String() string {
	return s.Formula()
}

func (s *ExponentialScale) Formula() string {
	return fmt.Sprintf("f(x) = %f * %f ^ (x - %f)", s.a, s.b, s.h)
}

func (s *ExponentialScale) FormulaOf(x float64) string {
	return fmt.Sprintf("f(%f) = %f * %f ^ (%f - %f)", x, s.a, s.b, x, s.h)
}

func (s *ExponentialScale) Call(x float64) (y float64) {
	if x < s.Domain[0] {
		x = s.Domain[0]
	} else if x > s.Domain[1] {
		x = s.Domain[1]
	}

	y = s.a * math.Pow(s.Range[1]/s.Range[0], (x-s.h)/s.s)
	return y
}

type LogarithmicScale struct {
	Domain [2]float64 `json:"domain"`
	Range  [2]float64 `json:"range"`

	h float64
	s float64
	a float64
}

func (s *LogarithmicScale) Call(x float64) (y float64) {
	if x < s.Domain[0] {
		x = s.Domain[0]
	} else if x > s.Domain[1] {
		x = s.Domain[1]
	}

	// y = a * log(x - h) + s
	y = s.a*math.Log(x-s.h) + s.s
	return y
}

func (s *LogarithmicScale) Sum(step float64) float64 {
	sum := 0.0
	for x := s.Domain[0]; x <= s.Domain[1]; x += step {
		sum += s.Call(x)
	}
	return sum
}

func (s *LogarithmicScale) String() string {
	return s.Formula()
}

func (s *LogarithmicScale) Formula() string {
	return fmt.Sprintf("f(x) = %f * log(x - %f) + %f", s.a, s.h, s.s)
}

func (s *LogarithmicScale) FormulaOf(x float64) string {
	return fmt.Sprintf("f(%f) = %f * log(%f - %f) + %f", x, s.a, x, s.h, s.s)
}

func (s *LogarithmicScale) Solve() error {
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

type LinearScale struct {
	Domain [2]float64 `json:"domain"`
	Range  [2]float64 `json:"range"`

	// a is the ratio for Range to Domain
	a float64
}

func (s *LinearScale) Solve() error {
	xs := s.Domain
	ys := s.Range

	s.a = (ys[1] - ys[0]) / (xs[1] - xs[0])

	return nil
}

func (s *LinearScale) Call(x float64) (y float64) {
	if x <= s.Domain[0] {
		return s.Range[0]
	} else if x >= s.Domain[1] {
		return s.Range[1]
	}

	y = s.Range[0] + (x-s.Domain[0])*s.a
	return y
}

func (s *LinearScale) Sum(step float64) float64 {
	sum := 0.0
	for x := s.Domain[0]; x <= s.Domain[1]; x += step {
		sum += s.Call(x)
	}
	return sum
}

func (s *LinearScale) String() string {
	return s.Formula()
}

func (s *LinearScale) Formula() string {
	return fmt.Sprintf("f(x) = %f + (x - %f) * %f", s.Range[0], s.Domain[0], s.a)
}

func (s *LinearScale) FormulaOf(x float64) string {
	return fmt.Sprintf("f(%f) = %f + (%f - %f) * %f", x, s.Range[0], x, s.Domain[0], s.a)
}

// see also: http://www.vb-helper.com/howto_find_quadratic_curve.html
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
	if x < s.Domain[0] {
		x = s.Domain[0]
	} else if x > s.Domain[2] {
		x = s.Domain[2]
	}

	// y = a * log(x - h) + s
	y = s.a*math.Pow(x, 2) + s.b*x + s.c
	return y
}

func (s *QuadraticScale) Sum(step float64) float64 {
	sum := 0.0
	for x := s.Domain[0]; x <= s.Domain[1]; x += step {
		sum += s.Call(x)
	}
	return sum
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

type SlideRule struct {
	// Scale type could be one of "log", "exp", "linear", "quadratic"
	// this is similar to the d3.scale
	LinearScale    *LinearScale      `json:"linear"`
	LogScale       *LogarithmicScale `json:"log"`
	ExpScale       *ExponentialScale `json:"exp"`
	QuadraticScale *QuadraticScale   `json:"quadratic"`
}

func (rule *SlideRule) Range() ([2]float64, error) {
	if rule.LogScale != nil {
		return rule.LogScale.Range, nil
	}

	if rule.ExpScale != nil {
		return rule.ExpScale.Range, nil
	}

	if rule.LinearScale != nil {
		return rule.LinearScale.Range, nil
	}

	if rule.QuadraticScale != nil {
		r := rule.QuadraticScale.Range
		return [2]float64{r[0], r[len(r)-1]}, nil
	}

	return [2]float64{}, errors.New("no any scale domain is defined")
}

func (rule *SlideRule) Scale() (Scale, error) {
	if rule.LogScale != nil {
		return rule.LogScale, nil
	}

	if rule.ExpScale != nil {
		return rule.ExpScale, nil
	}

	if rule.LinearScale != nil {
		return rule.LinearScale, nil
	}

	if rule.QuadraticScale != nil {
		return rule.QuadraticScale, nil
	}

	return nil, errors.New("no any scale is defined")
}

// LayerScale defines the scale DSL for maker layers, e.g.,
//
// quantityScale:
//
//	byLayer:
//	  exp:
//	    domain: [1, 5]
//	    range: [0.01, 1.0]
//
// and
//
// quantityScale:
//
//	byLayer:
//	  linear:
//	    domain: [1, 3]
//	    range: [0.01, 1.0]
type LayerScale struct {
	LayerRule *SlideRule `json:"byLayer"`
}

func (s *LayerScale) UnmarshalJSON(data []byte) error {
	type T LayerScale
	var p T
	err := json.Unmarshal(data, &p)
	if err != nil {
		return err
	}

	*s = LayerScale(p)
	return nil
}

func (s *LayerScale) Scale(layer int) (quantity float64, err error) {
	if s.LayerRule == nil {
		err = errors.New("either price or volume scale is not defined")
		return
	}

	scale, err := s.LayerRule.Scale()
	if err != nil {
		return 0, err
	}

	if err := scale.Solve(); err != nil {
		return 0, err
	}

	return scale.Call(float64(layer)), nil
}

// PriceVolumeScale defines the scale DSL for strategy, e.g.,
//
// quantityScale:
//
//	byPrice:
//	  exp:
//	    domain: [10_000, 50_000]
//	    range: [0.01, 1.0]
//
// and
//
// quantityScale:
//
//	byVolume:
//	  linear:
//	    domain: [10_000, 50_000]
//	    range: [0.01, 1.0]
type PriceVolumeScale struct {
	ByPriceRule  *SlideRule `json:"byPrice"`
	ByVolumeRule *SlideRule `json:"byVolume"`
}

func (s *PriceVolumeScale) Scale(price float64, volume float64) (quantity float64, err error) {
	if s.ByPriceRule != nil {
		quantity, err = s.ScaleByPrice(price)
		return
	} else if s.ByVolumeRule != nil {
		quantity, err = s.ScaleByVolume(volume)
	} else {
		err = errors.New("either price or volume scale is not defined")
	}
	return
}

// ScaleByPrice scale quantity by the given price
func (s *PriceVolumeScale) ScaleByPrice(price float64) (float64, error) {
	if s.ByPriceRule == nil {
		return 0, errors.New("byPrice scale is not defined")
	}

	scale, err := s.ByPriceRule.Scale()
	if err != nil {
		return 0, err
	}

	if err := scale.Solve(); err != nil {
		return 0, err
	}

	return scale.Call(price), nil
}

// ScaleByVolume scale quantity by the given volume
func (s *PriceVolumeScale) ScaleByVolume(volume float64) (float64, error) {
	if s.ByVolumeRule == nil {
		return 0, errors.New("byVolume scale is not defined")
	}

	scale, err := s.ByVolumeRule.Scale()
	if err != nil {
		return 0, err
	}

	if err := scale.Solve(); err != nil {
		return 0, err
	}

	return scale.Call(volume), nil
}

type PercentageScale struct {
	ByPercentage *SlideRule `json:"byPercentage"`
}

func (s *PercentageScale) Scale(percentage float64) (float64, error) {
	if s.ByPercentage == nil {
		return 0.0, errors.New("percentage scale is not defined")
	}

	scale, err := s.ByPercentage.Scale()
	if err != nil {
		return 0.0, err
	}

	if err := scale.Solve(); err != nil {
		return 0.0, err
	}

	return scale.Call(percentage), nil
}
