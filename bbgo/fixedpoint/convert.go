package fixedpoint

import (
	"math"
	"strconv"
)

const DefaultPrecision = 8

const DefaultPow = 1e8

type Value int64

func (v Value) Float64() float64 {
	return float64(v) / DefaultPow
}

func (v Value) Int64() int64 {
	return int64(v)
}

func (v Value) Mul(v2 Value) Value {
	return NewFromFloat(v.Float64() * v2.Float64())
}

func (v Value) Div(v2 Value) Value {
	return NewFromFloat(v.Float64() / v2.Float64())
}

func (v Value) Sub(v2 Value) Value {
	return Value(int64(v) - int64(v2))
}

func (v Value) Add(v2 Value) Value {
	return Value(int64(v) + int64(v2))
}

func NewFromString(input string) (Value, error) {
	v, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return 0, err
	}

	return NewFromFloat(v), nil
}

func NewFromFloat(val float64) Value {
	return Value(int64(math.Round(val * DefaultPow)))
}

func NewFromInt(val int) Value {
	return Value(int64(val * DefaultPow))
}

func NewFromInt64(val int64) Value {
	return Value(val * DefaultPow)
}
