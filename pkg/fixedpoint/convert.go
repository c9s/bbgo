package fixedpoint

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"sync/atomic"
)

const DefaultPrecision = 8

const DefaultPow = 1e8

type Value int64

func (v Value) Value() (driver.Value, error) {
	return v.Float64(), nil
}

func (v *Value) Scan(src interface{}) error {
	switch d := src.(type) {
	case int64:
		*v = Value(d)
		return nil

	case float64:
		*v = NewFromFloat(d)
		return nil

	case []byte:
		vv, err := NewFromString(string(d))
		if err != nil {
			return err
		}
		*v = vv
		return nil

	default:

	}

	return fmt.Errorf("fixedpoint.Value scan error, type: %T is not supported, value; %+v", src, src)
}

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

func (v *Value) AtomicAdd(v2 Value) {
	atomic.AddInt64((*int64)(v), int64(v2))
}

func (v *Value) AtomicLoad() Value {
	i := atomic.LoadInt64((*int64)(v))
	return Value(i)
}

func (v *Value) UnmarshalYAML(unmarshal func(a interface{}) error) (err error) {
	var f float64
	if err = unmarshal(&f); err == nil {
		*v = NewFromFloat(f)
		return
	}

	var i int64
	if err = unmarshal(&i); err == nil {
		*v = NewFromInt64(i)
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

func (v Value) MarshalJSON() ([]byte, error) {
	f := float64(v) / DefaultPow
	o := strconv.FormatFloat(f, 'f', 8, 64)
	return []byte(o), nil
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

	case string:
		v2, err := NewFromString(d)
		if err != nil {
			return err
		}

		*v = v2

	default:
		return fmt.Errorf("unsupported type: %T %v", d, d)

	}

	return nil
}

func Must(v Value, err error) Value {
	if err != nil {
		panic(err)
	}

	return v
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

func Min(a, b Value) Value {
	if a < b {
		return a
	}

	return b
}

func Max(a, b Value) Value {
	if a > b {
		return a
	}

	return b
}
