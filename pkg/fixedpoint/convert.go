package fixedpoint

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"sync/atomic"
)

const MaxPrecision = 12
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

func (v Value) String() string {
	return strconv.FormatFloat(float64(v)/DefaultPow, 'f', -1, 64)
}

func (v Value) Int64() int64 {
	return int64(v.Float64())
}

func (v Value) Int() int {
	return int(v.Float64())
}

// BigMul is the math/big version multiplication
func (v Value) BigMul(v2 Value) Value {
	x := new(big.Int).Mul(big.NewInt(int64(v)), big.NewInt(int64(v2)))
	return Value(x.Int64() / DefaultPow)
}

func (v Value) Mul(v2 Value) Value {
	return NewFromFloat(v.Float64() * v2.Float64())
}

func (v Value) MulInt(v2 int) Value {
	return NewFromFloat(v.Float64() * float64(v2))
}

func (v Value) MulFloat64(v2 float64) Value {
	return NewFromFloat(v.Float64() * v2)
}

func (v Value) Div(v2 Value) Value {
	return NewFromFloat(v.Float64() / v2.Float64())
}

func (v Value) DivFloat64(v2 float64) Value {
	return NewFromFloat(v.Float64() / v2)
}

func (v Value) Floor() Value {
	return NewFromFloat(math.Floor(v.Float64()))
}

func (v Value) Ceil() Value {
	return NewFromFloat(math.Ceil(v.Float64()))
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

var ErrPrecisionLoss = errors.New("precision loss")

func Parse(input string) (num int64, numDecimalPoints int, err error) {
	length := len(input)
	isPercentage := input[length-1] == '%'
	if isPercentage {
		length -= 1
		input = input[0:length]
	}

	var neg int64 = 1
	var digit int64
	for i := 0; i < length; i++ {
		c := input[i]
		if c == '-' {
			neg = -1
		} else if c >= '0' && c <= '9' {
			digit, err = strconv.ParseInt(string(c), 10, 64)
			if err != nil {
				return
			}

			num = num*10 + digit
		} else if c == '.' {
			i++
			if i > len(input)-1 {
				err = fmt.Errorf("expect fraction numbers after dot")
				return
			}

			for j := i; j < len(input); j++ {
				fc := input[j]
				if fc >= '0' && fc <= '9' {
					digit, err = strconv.ParseInt(string(fc), 10, 64)
					if err != nil {
						return
					}

					numDecimalPoints++
					num = num*10 + digit

					if numDecimalPoints >= MaxPrecision {
						return num, numDecimalPoints, ErrPrecisionLoss
					}
				} else {
					err = fmt.Errorf("expect digit, got %c", fc)
					return
				}
			}
			break
		} else {
			err = fmt.Errorf("unexpected char %c", c)
			return
		}
	}

	num = num * neg
	if isPercentage {
		numDecimalPoints += 2
	}

	return num, numDecimalPoints, nil
}

func NewFromString(input string) (Value, error) {
	length := len(input)

	if length == 0 {
		return 0, nil
	}

	isPercentage := input[length-1] == '%'
	if isPercentage {
		input = input[0 : length-1]
	}

	v, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return 0, err
	}

	if isPercentage {
		v = v * 0.01
	}

	return NewFromFloat(v), nil
}

func MustNewFromString(input string) Value {
	v, err := NewFromString(input)
	if err != nil {
		panic(fmt.Errorf("can not parse %s into fixedpoint, error: %s", input, err.Error()))
	}
	return v
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

func NumFractionalDigits(a Value) int {
	numPow := 0
	for pow := int64(DefaultPow); pow%10 != 1; pow /= 10 {
		numPow++
	}
	numZeros := 0
	for v := int64(a); v%10 == 0; v /= 10 {
		numZeros++
	}
	return numPow - numZeros
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

func Abs(a Value) Value {
	if a < 0 {
		return -a
	}
	return a
}
