package fixedpoint

import (
	"encoding/json"
	"math"
	"strconv"

	"github.com/pkg/errors"
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

func (v Value) MulFloat64(v2 float64) Value {
	return NewFromFloat(v.Float64() * v2)
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

func (v *Value) UnmarshalYAML(unmarshal func(a interface{}) error) (err error) {
	var i int64
	if err = unmarshal(&i); err == nil {
		*v = NewFromInt64(i)
		return
	}

	var f float64
	if err = unmarshal(&f); err == nil {
		*v = NewFromFloat(f)
		return
	}

	var s string
	if err = unmarshal(&s); err == nil {
		nv, err2 := NewFromString(s)
		if err2 == nil {
			*v = nv
			return
		}
	}

	return err
}

func (v *Value) UnmarshalJSON(data []byte) error {
	var a interface{}
	var err = json.Unmarshal(data, &a)
	if err != nil {
		return err
	}

	switch d := a.(type) {
	case float64:
		*v = NewFromFloat(d)

	case float32:
		*v = NewFromFloat32(d)

	case int:
		*v = NewFromInt(d)
	case int64:
		*v = NewFromInt64(d)

	default:
		return errors.Errorf("unsupported type: %T %v", d, d)

	}

	return nil
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

func NewFromFloat32(val float32) Value {
	return Value(int64(math.Round(float64(val) * DefaultPow)))
}

func NewFromInt(val int) Value {
	return Value(int64(val * DefaultPow))
}

func NewFromInt64(val int64) Value {
	return Value(val * DefaultPow)
}
