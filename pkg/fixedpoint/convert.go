//go:build !dnum

package fixedpoint

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
)

const MaxPrecision = 12
const DefaultPrecision = 8

const DefaultPow = 1e8

type Value int64

const Zero = Value(0)
const One = Value(1e8)
const NegOne = Value(-1e8)
const PosInf = Value(math.MaxInt64)
const NegInf = Value(math.MinInt64)

type RoundingMode int

const (
	Up RoundingMode = iota
	Down
	HalfUp
)

// Trunc returns the integer portion (truncating any fractional part)
func (v Value) Trunc() Value {
	return NewFromFloat(math.Floor(v.Float64()))
}

func (v Value) Round(r int, mode RoundingMode) Value {
	pow := math.Pow10(r)
	f := v.Float64() * pow
	switch mode {
	case Up:
		f = math.Ceil(f) / pow
	case HalfUp:
		f = math.Floor(f+0.5) / pow
	case Down:
		f = math.Floor(f) / pow
	}

	s := strconv.FormatFloat(f, 'f', r, 64)
	return MustNewFromString(s)
}

func (v Value) Value() (driver.Value, error) {
	return v.Float64(), nil
}

func (v *Value) Scan(src interface{}) error {
	switch d := src.(type) {
	case int64:
		*v = NewFromInt(d)
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
	if v == PosInf {
		return math.Inf(1)
	} else if v == NegInf {
		return math.Inf(-1)
	}
	return float64(v) / DefaultPow
}

func (v Value) Abs() Value {
	if v < 0 {
		return -v
	}
	return v
}

func (v Value) String() string {
	if v == PosInf {
		return "inf"
	} else if v == NegInf {
		return "-inf"
	}
	return strconv.FormatFloat(float64(v)/DefaultPow, 'f', -1, 64)
}

func (v Value) FormatString(prec int) string {
	if v == PosInf {
		return "inf"
	} else if v == NegInf {
		return "-inf"
	}

	u := int64(v)

	// trunc precision
	precDiff := DefaultPrecision - prec
	if precDiff > 0 {
		powDiff := int64(math.Round(math.Pow10(precDiff)))
		u = int64(v) / powDiff * powDiff
	}

	// check sign
	sign := Value(u).Sign()

	basePow := int64(DefaultPow)
	a := u / basePow
	b := u % basePow

	if a < 0 {
		a = -a
	}

	if b < 0 {
		b = -b
	}

	str := strconv.FormatInt(a, 10)
	if prec > 0 {
		bStr := fmt.Sprintf(".%08d", b)
		if prec <= DefaultPrecision {
			bStr = bStr[0 : prec+1]
		} else {
			for i := prec - DefaultPrecision; i > 0; i-- {
				bStr += "0"
			}
		}

		str += bStr
	}

	if sign < 0 {
		str = "-" + str
	}

	return str
}

func (v Value) Percentage() string {
	if v == 0 {
		return "0"
	}
	if v == PosInf {
		return "inf%"
	} else if v == NegInf {
		return "-inf%"
	}
	return strconv.FormatFloat(float64(v)/DefaultPow*100., 'f', -1, 64) + "%"
}

func (v Value) FormatPercentage(prec int) string {
	if v == 0 {
		return "0"
	}
	if v == PosInf {
		return "inf%"
	} else if v == NegInf {
		return "-inf%"
	}
	pow := math.Pow10(prec)
	result := strconv.FormatFloat(
		math.Trunc(float64(v)/DefaultPow*pow*100.)/pow, 'f', prec, 64)
	return result + "%"
}

func (v Value) SignedPercentage() string {
	if v > 0 {
		return "+" + v.Percentage()
	}
	return v.Percentage()
}

func (v Value) Int64() int64 {
	return int64(v.Float64())
}

func (v Value) Int() int {
	n := v.Int64()
	if int64(int(n)) != n {
		panic("unable to convert Value to int32")
	}
	return int(n)
}

func (v Value) Neg() Value {
	return -v
}

// TODO inf
func (v Value) Sign() int {
	if v > 0 {
		return 1
	} else if v == 0 {
		return 0
	} else {
		return -1
	}
}

func (v Value) IsZero() bool {
	return v == 0
}

func Mul(x, y Value) Value {
	return NewFromFloat(x.Float64() * y.Float64())
}

func (v Value) Mul(v2 Value) Value {
	return NewFromFloat(v.Float64() * v2.Float64())
}

func Div(x, y Value) Value {
	return NewFromFloat(x.Float64() / y.Float64())
}

func (v Value) Div(v2 Value) Value {
	return NewFromFloat(v.Float64() / v2.Float64())
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

func (v Value) MarshalYAML() (interface{}, error) {
	return v.FormatString(DefaultPrecision), nil
}

func (v Value) MarshalJSON() ([]byte, error) {
	if v.IsInf() {
		return []byte("\"" + v.String() + "\""), nil
	}
	return []byte(v.FormatString(DefaultPrecision)), nil
}

func (v *Value) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte{'n', 'u', 'l', 'l'}) {
		*v = Zero
		return nil
	}
	if len(data) == 0 {
		*v = Zero
		return nil
	}
	var err error
	if data[0] == '"' {
		data = data[1 : len(data)-1]
	}
	if *v, err = NewFromString(string(data)); err != nil {
		return err
	}
	return nil
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
	dotIndex := -1
	hasDecimal := false
	decimalCount := 0
	// if is decimal, we don't need this
	hasScientificNotion := false
	hasIChar := false
	scIndex := -1
	for i, c := range input {
		if hasDecimal {
			if c <= '9' && c >= '0' {
				decimalCount++
			} else {
				break
			}

		} else if c == '.' {
			dotIndex = i
			hasDecimal = true
		}
		if c == 'e' || c == 'E' {
			hasScientificNotion = true
			scIndex = i
			break
		}
		if c == 'i' || c == 'I' {
			hasIChar = true
			break
		}
	}
	if hasDecimal {
		after := input[dotIndex+1:]
		if decimalCount >= 8 {
			after = after[0:8] + "." + after[8:]
		} else {
			after = after[0:decimalCount] + strings.Repeat("0", 8-decimalCount) + after[decimalCount:]
		}
		input = input[0:dotIndex] + after
		v, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return 0, err
		}

		if isPercentage {
			v = v * 0.01
		}

		return Value(int64(math.Trunc(v))), nil

	} else if hasScientificNotion {
		exp, err := strconv.ParseInt(input[scIndex+1:], 10, 32)
		if err != nil {
			return 0, err
		}
		v, err := strconv.ParseFloat(input[0:scIndex+1]+strconv.FormatInt(exp+8, 10), 64)
		if err != nil {
			return 0, err
		}
		return Value(int64(math.Trunc(v))), nil
	} else if hasIChar {
		if floatV, err := strconv.ParseFloat(input, 64); nil != err {
			return 0, err
		} else if math.IsInf(floatV, 1) {
			return PosInf, nil
		} else if math.IsInf(floatV, -1) {
			return NegInf, nil
		} else {
			return 0, fmt.Errorf("fixedpoint.Value parse error, invalid input string %s", input)
		}
	} else {
		v, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			return 0, err
		}
		if isPercentage {
			v = v * DefaultPow / 100
		} else {
			v = v * DefaultPow
		}
		return Value(v), nil
	}
}

func MustNewFromString(input string) Value {
	v, err := NewFromString(input)
	if err != nil {
		panic(fmt.Errorf("can not parse %s into fixedpoint, error: %s", input, err.Error()))
	}
	return v
}

func NewFromBytes(input []byte) (Value, error) {
	return NewFromString(string(input))
}

func MustNewFromBytes(input []byte) (v Value) {
	var err error
	if v, err = NewFromString(string(input)); err != nil {
		return Zero
	}
	return v
}

func Must(v Value, err error) Value {
	if err != nil {
		panic(err)
	}
	return v
}

func NewFromFloat(val float64) Value {
	if math.IsInf(val, 1) {
		return PosInf
	} else if math.IsInf(val, -1) {
		return NegInf
	}
	return Value(int64(math.Trunc(val * DefaultPow)))
}

func NewFromInt(val int64) Value {
	return Value(val * DefaultPow)
}

func (a Value) IsInf() bool {
	return a == PosInf || a == NegInf
}

func (a Value) MulExp(exp int) Value {
	return Value(int64(float64(a) * math.Pow(10, float64(exp))))
}

func (a Value) NumIntDigits() int {
	digits := 0
	target := int64(a)
	for pow := int64(DefaultPow); pow <= target; pow *= 10 {
		digits++
	}
	return digits
}

// TODO: speedup
func (a Value) NumFractionalDigits() int {
	if a == 0 {
		return 0
	}
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

func Compare(x, y Value) int {
	if x > y {
		return 1
	} else if x == y {
		return 0
	} else {
		return -1
	}
}

func (x Value) Compare(y Value) int {
	if x > y {
		return 1
	} else if x == y {
		return 0
	} else {
		return -1
	}
}

func Min(a, b Value) Value {
	if a.Compare(b) < 0 {
		return a
	}

	return b
}

func Max(a, b Value) Value {
	if a.Compare(b) > 0 {
		return a
	}

	return b
}

func Equal(x, y Value) bool {
	return x == y
}

func (x Value) Eq(y Value) bool {
	return x == y
}

func Abs(a Value) Value {
	if a < 0 {
		return -a
	}
	return a
}

func Clamp(x, min, max Value) Value {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}

func (x Value) Clamp(min, max Value) Value {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}
