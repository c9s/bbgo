//go:build dnum

package fixedpoint

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
)

type Value struct {
	coef uint64
	sign int8
	exp  int
}

const (
	signPosInf = +2
	signPos    = +1
	signZero   = 0
	signNeg    = -1
	signNegInf = -2
	coefMin    = 1000_0000_0000_0000
	coefMax    = 9999_9999_9999_9999
	digitsMax  = 16
	shiftMax   = digitsMax - 1
	// to switch between scientific notion and normal presentation format
	maxLeadingZeros = 19
)

// common values
var (
	Zero   = Value{}
	One    = Value{1000_0000_0000_0000, signPos, 1}
	NegOne = Value{1000_0000_0000_0000, signNeg, 1}
	PosInf = Value{1, signPosInf, 0}
	NegInf = Value{1, signNegInf, 0}
)

var pow10f = [...]float64{
	1,
	10,
	100,
	1000,
	10000,
	100000,
	1000000,
	10000000,
	100000000,
	1000000000,
	10000000000,
	100000000000,
	1000000000000,
	10000000000000,
	100000000000000,
	1000000000000000,
	10000000000000000,
	100000000000000000,
	1000000000000000000,
	10000000000000000000,
	100000000000000000000}

var pow10 = [...]uint64{
	1,
	10,
	100,
	1000,
	10000,
	100000,
	1000000,
	10000000,
	100000000,
	1000000000,
	10000000000,
	100000000000,
	1000000000000,
	10000000000000,
	100000000000000,
	1000000000000000,
	10000000000000000,
	100000000000000000,
	1000000000000000000}

var halfpow10 = [...]uint64{
	0,
	5,
	50,
	500,
	5000,
	50000,
	500000,
	5000000,
	50000000,
	500000000,
	5000000000,
	50000000000,
	500000000000,
	5000000000000,
	50000000000000,
	500000000000000,
	5000000000000000,
	50000000000000000,
	500000000000000000,
	5000000000000000000}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func (v Value) Value() (driver.Value, error) {
	return v.Float64(), nil
}

// NewFromInt returns a Value for an int
func NewFromInt(n int64) Value {
	if n == 0 {
		return Zero
	}
	// n0 := n
	sign := int8(signPos)
	if n < 0 {
		n = -n
		sign = signNeg
	}
	return newNoSignCheck(sign, uint64(n), digitsMax)
}

const log2of10 = 3.32192809488736234

// NewFromFloat converts a float64 to a Value
func NewFromFloat(f float64) Value {
	switch {
	case math.IsInf(f, +1):
		return PosInf
	case math.IsInf(f, -1):
		return NegInf
	case math.IsNaN(f):
		panic("value.NewFromFloat can't convert NaN")
	}

	if f == 0 {
		return Zero
	}

	sign := int8(signPos)
	if f < 0 {
		f = -f
		sign = signNeg
	}
	n := uint64(f)
	if float64(n) == f {
		return newNoSignCheck(sign, n, digitsMax)
	}
	_, e := math.Frexp(f)
	e = int(float32(e) / log2of10)
	c := uint64(f/math.Pow10(e-16) + 0.5)
	return newNoSignCheck(sign, c, e)
}

// Raw constructs a Value without normalizing - arguments must be valid.
// Used by SuValue Unpack
func Raw(sign int8, coef uint64, exp int) Value {
	return Value{coef, sign, int(exp)}
}

func newNoSignCheck(sign int8, coef uint64, exp int) Value {
	atmax := false
	for coef > coefMax {
		coef = (coef + 5) / 10
		exp++
		atmax = true
	}

	if !atmax {
		p := maxShift(coef)
		coef *= pow10[p]
		exp -= p
	}
	return Value{coef, sign, exp}
}

// New constructs a Value, maximizing coef and handling exp out of range
// Used to normalize results of operations
func New(sign int8, coef uint64, exp int) Value {
	if sign == 0 || coef == 0 {
		return Zero
	} else if sign == signPosInf {
		return PosInf
	} else if sign == signNegInf {
		return NegInf
	} else {
		atmax := false
		for coef > coefMax {
			coef = (coef + 5) / 10
			exp++
			atmax = true
		}

		if !atmax {
			p := maxShift(coef)
			coef *= pow10[p]
			exp -= p
		}
		return Value{coef, sign, exp}
	}
}

func maxShift(x uint64) int {
	i := ilog10(x)
	if i > shiftMax {
		return 0
	}
	return shiftMax - i
}

func ilog10(x uint64) int {
	// based on Hacker's Delight
	if x == 0 {
		return 0
	}
	y := (19 * (63 - bits.LeadingZeros64(x))) >> 6
	if y < 18 && x >= pow10[y+1] {
		y++
	}
	return y
}

func Inf(sign int8) Value {
	switch {
	case sign < 0:
		return NegInf
	case sign > 0:
		return PosInf
	default:
		return Zero
	}
}

func (dn Value) FormatString(prec int) string {
	if dn.sign == 0 {
		if prec <= 0 {
			return "0"
		} else {
			return "0." + strings.Repeat("0", prec)
		}
	}
	sign := ""
	if dn.sign < 0 {
		sign = "-"
	}
	if dn.IsInf() {
		return sign + "inf"
	}
	digits := getDigits(dn.coef)
	nd := len(digits)
	e := int(dn.exp) - nd
	if -maxLeadingZeros <= dn.exp && dn.exp <= 0 {
		if prec < 0 {
			return "0"
		}
		// decimal to the left
		if prec+e+nd > 0 {
			return sign + "0." + strings.Repeat("0", -e-nd) + digits[:min(prec+e+nd, nd)] + strings.Repeat("0", max(0, prec-nd+e+nd))
		} else if -e-nd > 0 && prec != 0 {
			return "0." + strings.Repeat("0", min(prec, -e-nd))
		} else {
			return "0"
		}
	} else if -nd < e && e <= -1 {
		// decimal within
		dec := nd + e
		if prec > 0 {
			decimals := digits[dec:min(dec+prec, nd)]
			return sign + digits[:dec] + "." + decimals + strings.Repeat("0", max(0, prec-len(decimals)))
		} else if prec == 0 {
			return sign + digits[:dec]
		}

		sigFigures := digits[0:max(dec+prec, 0)]
		if len(sigFigures) == 0 {
			return "0"
		}

		return sign + sigFigures + strings.Repeat("0", max(-prec, 0))

	} else if 0 < dn.exp && dn.exp <= digitsMax {
		// decimal to the right
		if prec > 0 {
			return sign + digits + strings.Repeat("0", e) + "." + strings.Repeat("0", prec)
		} else if prec+e >= 0 {
			return sign + digits + strings.Repeat("0", e)
		} else {
			if len(digits) <= -prec-e {
				return "0"
			}

			return sign + digits[0:len(digits)+prec+e] + strings.Repeat("0", -prec)
		}
	} else {
		// scientific notation
		after := ""
		if nd > 1 {
			after = "." + digits[1:min(1+prec, nd)] + strings.Repeat("0", max(0, min(1+prec, nd)-1-prec))
		}
		return sign + digits[:1] + after + "e" + strconv.Itoa(int(dn.exp-1))
	}
}

// String returns a string representation of the Value
func (dn Value) String() string {
	if dn.sign == 0 {
		return "0"
	}
	sign := ""
	if dn.sign < 0 {
		sign = "-"
	}
	if dn.IsInf() {
		return sign + "inf"
	}
	digits := getDigits(dn.coef)
	nd := len(digits)
	e := int(dn.exp) - nd
	if -maxLeadingZeros <= dn.exp && dn.exp <= 0 {
		// decimal to the left
		return sign + "0." + strings.Repeat("0", -e-nd) + digits
	} else if -nd < e && e <= -1 {
		// decimal within
		dec := nd + e
		return sign + digits[:dec] + "." + digits[dec:]
	} else if 0 < dn.exp && dn.exp <= digitsMax {
		// decimal to the right
		return sign + digits + strings.Repeat("0", e)
	} else {
		// scientific notation
		after := ""
		if nd > 1 {
			after = "." + digits[1:]
		}
		return sign + digits[:1] + after + "e" + strconv.Itoa(int(dn.exp-1))
	}
}

func (dn Value) Percentage() string {
	if dn.sign == 0 {
		return "0%"
	}
	sign := ""
	if dn.sign < 0 {
		sign = "-"
	}
	if dn.IsInf() {
		return sign + "inf%"
	}
	digits := getDigits(dn.coef)
	nd := len(digits)
	e := int(dn.exp) - nd + 2

	if -maxLeadingZeros <= dn.exp && dn.exp <= -2 {
		// decimal to the left
		return sign + "0." + strings.Repeat("0", -e-nd) + digits + "%"
	} else if -nd < e && e <= -1 {
		// decimal within
		dec := nd + e
		return sign + digits[:dec] + "." + digits[dec:] + "%"
	} else if -2 < dn.exp && dn.exp <= digitsMax {
		// decimal to the right
		return sign + digits + strings.Repeat("0", e) + "%"
	} else {
		// scientific notation
		after := ""
		if nd > 1 {
			after = "." + digits[1:]
		}
		return sign + digits[:1] + after + "e" + strconv.Itoa(int(dn.exp-1)) + "%"
	}
}

func (dn Value) FormatPercentage(prec int) string {
	if dn.sign == 0 {
		if prec <= 0 {
			return "0"
		} else {
			return "0." + strings.Repeat("0", prec)
		}
	}
	sign := ""
	if dn.sign < 0 {
		sign = "-"
	}
	if dn.IsInf() {
		return sign + "inf"
	}
	digits := getDigits(dn.coef)
	nd := len(digits)
	exp := dn.exp + 2
	e := int(exp) - nd

	if -maxLeadingZeros <= exp && exp <= 0 {
		// decimal to the left
		if prec+e+nd > 0 {
			return sign + "0." + strings.Repeat("0", -e-nd) + digits[:min(prec+e+nd, nd)] + strings.Repeat("0", max(0, prec-nd+e+nd)) + "%"
		} else if -e-nd > 0 {
			return "0." + strings.Repeat("0", -e-nd) + "%"
		} else {
			return "0"
		}
	} else if -nd < e && e <= -1 {
		// decimal within
		dec := nd + e
		decimals := digits[dec:min(dec+prec, nd)]
		return sign + digits[:dec] + "." + decimals + strings.Repeat("0", max(0, prec-len(decimals))) + "%"
	} else if 0 < exp && exp <= digitsMax {
		// decimal to the right
		if prec > 0 {
			return sign + digits + strings.Repeat("0", e) + "." + strings.Repeat("0", prec) + "%"
		} else {
			return sign + digits + strings.Repeat("0", e) + "%"
		}
	} else {
		// scientific notation
		after := ""
		if nd > 1 {
			after = "." + digits[1:min(1+prec, nd)] + strings.Repeat("0", max(0, min(1+prec, nd)-1-prec))
		}
		return sign + digits[:1] + after + "e" + strconv.Itoa(int(exp-1)) + "%"
	}
}

func (dn Value) SignedPercentage() string {
	if dn.Sign() >= 0 {
		return "+" + dn.Percentage()
	}
	return dn.Percentage()
}

// get digit length
func (a Value) NumDigits() int {
	i := shiftMax
	coef := a.coef
	nd := 0
	for coef != 0 && coef < pow10[i] {
		i--
	}
	for coef != 0 {
		coef %= pow10[i]
		i--
		nd++
	}
	return nd
}

// alias of Exp
func (a Value) NumIntDigits() int {
	return a.exp
}

// get fractional digits
func (a Value) NumFractionalDigits() int {
	nd := a.NumDigits()
	return nd - a.exp
}

func getDigits(coef uint64) string {
	var digits [digitsMax]byte
	i := shiftMax
	nd := 0
	for coef != 0 {
		digits[nd] = byte('0' + (coef / pow10[i]))
		coef %= pow10[i]
		nd++
		i--
	}
	return string(digits[:nd])
}

func (v *Value) Scan(src interface{}) error {
	var err error
	switch d := src.(type) {
	case int64:
		*v = NewFromInt(d)
		return nil
	case float64:
		*v = NewFromFloat(d)
		return nil
	case []byte:
		*v, err = NewFromString(string(d))
		if err != nil {
			return err
		}
		return nil
	default:
	}
	return fmt.Errorf("fixedpoint.Value scan error, type %T is not supported, value: %+v", src, src)
}

// NewFromString parses a numeric string and returns a Value representation.
func NewFromString(s string) (Value, error) {
	length := len(s)
	if length == 0 {
		return Zero, nil
	}
	isPercentage := s[length-1] == '%'
	if isPercentage {
		s = s[:length-1]
	}
	r := &reader{s, 0}
	sign := r.getSign()
	if r.matchStrIgnoreCase("inf") {
		return Inf(sign), nil
	}
	coef, exp := r.getCoef()
	exp += r.getExp()
	if r.len() != 0 { // didn't consume entire string
		return Zero, errors.New("invalid number")
	} else if coef == 0 || exp < math.MinInt8 {
		return Zero, nil
	} else if exp > math.MaxInt8 {
		return Inf(sign), nil
	}
	if isPercentage {
		exp -= 2
	}
	atmax := false
	for coef > coefMax {
		coef = (coef + 5) / 10
		exp++
		atmax = true
	}

	if !atmax {
		p := maxShift(coef)
		coef *= pow10[p]
		exp -= p
	}
	// check(coefMin <= coef && coef <= coefMax)
	return Value{coef, sign, exp}, nil
}

func MustNewFromString(input string) Value {
	v, err := NewFromString(input)
	if err != nil {
		panic(fmt.Errorf("cannot parse %s into fixedpoint, error: %s", input, err.Error()))
	}
	return v
}

func NewFromBytes(s []byte) (Value, error) {
	length := len(s)
	if length == 0 {
		return Zero, nil
	}
	isPercentage := s[length-1] == '%'
	if isPercentage {
		s = s[:length-1]
	}
	r := &readerBytes{s, 0}
	sign := r.getSign()
	if r.matchStrIgnoreCase("inf") {
		return Inf(sign), nil
	}
	coef, exp := r.getCoef()
	exp += r.getExp()
	if r.len() != 0 { // didn't consume entire string
		return Zero, errors.New("invalid number")
	} else if coef == 0 || exp < math.MinInt8 {
		return Zero, nil
	} else if exp > math.MaxInt8 {
		return Inf(sign), nil
	}
	if isPercentage {
		exp -= 2
	}
	atmax := false
	for coef > coefMax {
		coef = (coef + 5) / 10
		exp++
		atmax = true
	}

	if !atmax {
		p := maxShift(coef)
		coef *= pow10[p]
		exp -= p
	}
	// check(coefMin <= coef && coef <= coefMax)
	return Value{coef, sign, exp}, nil
}

func MustNewFromBytes(input []byte) Value {
	v, err := NewFromBytes(input)
	if err != nil {
		panic(fmt.Errorf("cannot parse %s into fixedpoint, error: %s", input, err.Error()))
	}
	return v
}

// TODO: refactor by interface

type readerBytes struct {
	s []byte
	i int
}

func (r *readerBytes) cur() byte {
	if r.i >= len(r.s) {
		return 0
	}
	return byte(r.s[r.i])
}

func (r *readerBytes) prev() byte {
	if r.i == 0 {
		return 0
	}
	return byte(r.s[r.i-1])
}

func (r *readerBytes) len() int {
	return len(r.s) - r.i
}

func (r *readerBytes) match(c byte) bool {
	if r.cur() == c {
		r.i++
		return true
	}
	return false
}

func (r *readerBytes) matchDigit() bool {
	c := r.cur()
	if '0' <= c && c <= '9' {
		r.i++
		return true
	}
	return false
}

func (r *readerBytes) matchStrIgnoreCase(pre string) bool {
	pre = strings.ToLower(pre)
	boundary := r.i + len(pre)
	if boundary > len(r.s) {
		return false
	}
	for i, c := range bytes.ToLower(r.s[r.i:boundary]) {
		if pre[i] != c {
			return false
		}
	}
	r.i = boundary
	return true
}

func (r *readerBytes) getSign() int8 {
	if r.match('-') {
		return int8(signNeg)
	}
	r.match('+')
	return int8(signPos)
}

func (r *readerBytes) getCoef() (uint64, int) {
	digits := false
	beforeDecimal := true
	for r.match('0') {
		digits = true
	}
	if r.cur() == '.' && r.len() > 1 {
		digits = false
	}
	n := uint64(0)
	exp := 0
	p := shiftMax
	for {
		c := r.cur()
		if r.matchDigit() {
			digits = true
			// ignore extra decimal places
			if c != '0' && p >= 0 {
				n += uint64(c-'0') * pow10[p]
			}
			p--
		} else if beforeDecimal {
			// decimal point or end
			exp = shiftMax - p
			if !r.match('.') {
				break
			}
			beforeDecimal = false
			if !digits {
				for r.match('0') {
					digits = true
					exp--
				}
			}
		} else {
			break
		}
	}
	if !digits {
		panic("numbers require at least one digit")
	}
	return n, exp
}

func (r *readerBytes) getExp() int {
	e := 0
	if r.match('e') || r.match('E') {
		esign := r.getSign()
		for r.matchDigit() {
			e = e*10 + int(r.prev()-'0')
		}
		e *= int(esign)
	}
	return e
}

type reader struct {
	s string
	i int
}

func (r *reader) cur() byte {
	if r.i >= len(r.s) {
		return 0
	}
	return byte(r.s[r.i])
}

func (r *reader) prev() byte {
	if r.i == 0 {
		return 0
	}
	return byte(r.s[r.i-1])
}

func (r *reader) len() int {
	return len(r.s) - r.i
}

func (r *reader) match(c byte) bool {
	if r.cur() == c {
		r.i++
		return true
	}
	return false
}

func (r *reader) matchDigit() bool {
	c := r.cur()
	if '0' <= c && c <= '9' {
		r.i++
		return true
	}
	return false
}

func (r *reader) matchStrIgnoreCase(pre string) bool {
	boundary := r.i + len(pre)
	if boundary > len(r.s) {
		return false
	}
	data := strings.ToLower(r.s[r.i:boundary])
	pre = strings.ToLower(pre)
	if data == pre {
		r.i = boundary
		return true
	}
	return false
}

func (r *reader) getSign() int8 {
	if r.match('-') {
		return int8(signNeg)
	}
	r.match('+')
	return int8(signPos)
}

func (r *reader) getCoef() (uint64, int) {
	digits := false
	beforeDecimal := true
	for r.match('0') {
		digits = true
	}
	if r.cur() == '.' && r.len() > 1 {
		digits = false
	}
	n := uint64(0)
	exp := 0
	p := shiftMax
	for {
		c := r.cur()
		if r.matchDigit() {
			digits = true
			// ignore extra decimal places
			if c != '0' && p >= 0 {
				n += uint64(c-'0') * pow10[p]
			}
			p--
		} else if beforeDecimal {
			// decimal point or end
			exp = shiftMax - p
			if !r.match('.') {
				break
			}
			beforeDecimal = false
			if !digits {
				for r.match('0') {
					digits = true
					exp--
				}
			}
		} else {
			break
		}
	}
	if !digits {
		panic("numbers require at least one digit")
	}
	return n, exp
}

func (r *reader) getExp() int {
	e := 0
	if r.match('e') || r.match('E') {
		esign := r.getSign()
		for r.matchDigit() {
			e = e*10 + int(r.prev()-'0')
		}
		e *= int(esign)
	}
	return e
}

// end of FromStr ---------------------------------------------------

// IsInf returns true if a Value is positive or negative infinite
func (dn Value) IsInf() bool {
	return dn.sign == signPosInf || dn.sign == signNegInf
}

// IsZero returns true if a Value is zero
func (dn Value) IsZero() bool {
	return dn.sign == signZero
}

// Float64 converts a Value to float64
func (dn Value) Float64() float64 {
	if dn.IsInf() {
		return math.Inf(int(dn.sign))
	}
	g := float64(dn.coef)
	if dn.sign == signNeg {
		g = -g
	}
	i := int(dn.exp) - digitsMax
	return g * math.Pow(10, float64(i))
}

// Int64 converts a Value to an int64, returning whether it was convertible
func (dn Value) Int64() int64 {
	if dn.sign == 0 {
		return 0
	}
	if dn.sign != signNegInf && dn.sign != signPosInf {
		if 0 < dn.exp && dn.exp < digitsMax {
			return int64(dn.sign) * int64(dn.coef/pow10[digitsMax-dn.exp])
		} else if dn.exp <= 0 && dn.coef != 0 {
			result := math.Log10(float64(dn.coef)) - float64(digitsMax) + float64(dn.exp)
			return int64(dn.sign) * int64(math.Pow(10, result))
		}
		if dn.exp == digitsMax {
			return int64(dn.sign) * int64(dn.coef)
		}
		if dn.exp == digitsMax+1 {
			return int64(dn.sign) * (int64(dn.coef) * 10)
		}
		if dn.exp == digitsMax+2 {
			return int64(dn.sign) * (int64(dn.coef) * 100)
		}
		if dn.exp == digitsMax+3 && dn.coef < math.MaxInt64/1000 {
			return int64(dn.sign) * (int64(dn.coef) * 1000)
		}
	}
	panic("unable to convert Value to int64")
}

func (dn Value) Int() int {
	// if int is int64, this is a nop
	n := dn.Int64()
	if int64(int(n)) != n {
		panic("unable to convert Value to int32")
	}
	return int(n)
}

// Sign returns -1 for negative, 0 for zero, and +1 for positive
func (dn Value) Sign() int {
	return int(dn.sign)
}

// Coef returns the coefficient
func (dn Value) Coef() uint64 {
	return dn.coef
}

// Exp returns the exponent
func (dn Value) Exp() int {
	return int(dn.exp)
}

// Frac returns the fractional portion, i.e. x - x.Int()
func (dn Value) Frac() Value {
	if dn.sign == 0 || dn.sign == signNegInf || dn.sign == signPosInf ||
		dn.exp >= digitsMax {
		return Zero
	}
	if dn.exp <= 0 {
		return dn
	}
	frac := dn.coef % pow10[digitsMax-dn.exp]
	if frac == dn.coef {
		return dn
	}
	return New(dn.sign, frac, int(dn.exp))
}

type RoundingMode int

const (
	Up RoundingMode = iota
	Down
	HalfUp
)

// Trunc returns the integer portion (truncating any fractional part)
func (dn Value) Trunc() Value {
	return dn.integer(Down)
}

func (dn Value) integer(mode RoundingMode) Value {
	if dn.sign == 0 || dn.sign == signNegInf || dn.sign == signPosInf ||
		dn.exp >= digitsMax {
		return dn
	}
	if dn.exp <= 0 {
		if mode == Up ||
			(mode == HalfUp && dn.exp == 0 && dn.coef >= One.coef*5) {
			return New(dn.sign, One.coef, int(dn.exp)+1)
		}
		return Zero
	}
	e := digitsMax - dn.exp
	frac := dn.coef % pow10[e]
	if frac == 0 {
		return dn
	}
	i := dn.coef - frac
	if (mode == Up && frac > 0) || (mode == HalfUp && frac >= halfpow10[e]) {
		return New(dn.sign, i+pow10[e], int(dn.exp)) // normalize
	}
	return Value{i, dn.sign, dn.exp}
}

func (dn Value) Floor() Value {
	return dn.Round(0, Down)
}

func (dn Value) Round(r int, mode RoundingMode) Value {
	if dn.sign == 0 || dn.sign == signNegInf || dn.sign == signPosInf ||
		r >= digitsMax {
		return dn
	}
	if r <= -digitsMax {
		return Zero
	}
	n := New(dn.sign, dn.coef, int(dn.exp)+r) // multiply by 10^r
	n = n.integer(mode)
	if n.sign == signPos || n.sign == signNeg { // i.e. not zero or inf
		return New(n.sign, n.coef, int(n.exp)-r)
	}
	return n
}

// arithmetic operations -------------------------------------------------------

// Neg returns the Value negated i.e. sign reversed
func (dn Value) Neg() Value {
	return Value{dn.coef, -dn.sign, dn.exp}
}

// Abs returns the Value with a positive sign
func (dn Value) Abs() Value {
	if dn.sign < 0 {
		return Value{dn.coef, -dn.sign, dn.exp}
	}
	return dn
}

// Equal returns true if two Value's are equal
func Equal(x, y Value) bool {
	return x.sign == y.sign && x.exp == y.exp && x.coef == y.coef
}

func (x Value) Eq(y Value) bool {
	return Equal(x, y)
}

func Max(x, y Value) Value {
	if Compare(x, y) > 0 {
		return x
	}
	return y
}

func Min(x, y Value) Value {
	if Compare(x, y) < 0 {
		return x
	}
	return y
}

// Compare compares two Value's returning -1 for <, 0 for ==, +1 for >
func Compare(x, y Value) int {
	switch {
	case x.sign < y.sign:
		return -1
	case x.sign > y.sign:
		return 1
	case x == y:
		return 0
	}
	sign := int(x.sign)
	switch {
	case sign == 0 || sign == signNegInf || sign == signPosInf:
		return 0
	case x.exp < y.exp:
		return -sign
	case x.exp > y.exp:
		return +sign
	case x.coef < y.coef:
		return -sign
	case x.coef > y.coef:
		return +sign
	default:
		return 0
	}
}

func (x Value) Compare(y Value) int {
	return Compare(x, y)
}

func (v Value) MarshalYAML() (interface{}, error) {
	return v.FormatString(8), nil
}

func (v *Value) UnmarshalYAML(unmarshal func(a interface{}) error) (err error) {
	var f float64
	if err = unmarshal(&f); err == nil {
		*v = NewFromFloat(f)
		return
	}
	var i int64
	if err = unmarshal(&i); err == nil {
		*v = NewFromInt(i)
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

// FIXME: should we limit to 8 prec?
func (v Value) MarshalJSON() ([]byte, error) {
	if v.IsInf() {
		return []byte("\"" + v.String() + "\""), nil
	}
	return []byte(v.FormatString(8)), nil
}

func (v *Value) UnmarshalJSON(data []byte) error {
	// FIXME: do we need to compare {}, [], "", or "null"?
	if bytes.Compare(data, []byte{'n', 'u', 'l', 'l'}) == 0 {
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
	if *v, err = NewFromBytes(data); err != nil {
		return err
	}
	return nil
}

func Must(v Value, err error) Value {
	if err != nil {
		panic(err)
	}
	return v
}

// v * 10^(exp)
func (v Value) MulExp(exp int) Value {
	return Value{v.coef, v.sign, v.exp + exp}
}

// Sub returns the difference of two Value's
func Sub(x, y Value) Value {
	return Add(x, y.Neg())
}

func (x Value) Sub(y Value) Value {
	return Sub(x, y)
}

// Add returns the sum of two Value's
func Add(x, y Value) Value {
	switch {
	case x.sign == signZero:
		return y
	case y.sign == signZero:
		return x
	case x.IsInf():
		if y.sign == -x.sign {
			return Zero
		}
		return x
	case y.IsInf():
		return y
	}
	if !align(&x, &y) {
		return x
	}
	if x.sign != y.sign {
		return usub(x, y)
	}
	return uadd(x, y)
}

func (x Value) Add(y Value) Value {
	return Add(x, y)
}

func uadd(x, y Value) Value {
	return New(x.sign, x.coef+y.coef, int(x.exp))
}

func usub(x, y Value) Value {
	if x.coef < y.coef {
		return New(-x.sign, y.coef-x.coef, int(x.exp))
	}
	return New(x.sign, x.coef-y.coef, int(x.exp))
}

func align(x, y *Value) bool {
	if x.exp == y.exp {
		return true
	}
	if x.exp < y.exp {
		*x, *y = *y, *x // swap
	}
	yshift := ilog10(y.coef)
	e := int(x.exp - y.exp)
	if e > yshift {
		return false
	}
	yshift = e
	// check(0 <= yshift && yshift <= 20)
	// y.coef = (y.coef + halfpow10[yshift]) / pow10[yshift]
	y.coef = (y.coef) / pow10[yshift]
	// check(int(y.exp)+yshift == int(x.exp))
	return true
}

const e7 = 10000000

// Mul returns the product of two Value's
func Mul(x, y Value) Value {
	sign := x.sign * y.sign
	switch {
	case sign == signZero:
		return Zero
	case x.IsInf() || y.IsInf():
		return Inf(sign)
	}
	e := int(x.exp) + int(y.exp)

	// split unevenly to use full 64 bit range to get more precision
	// and avoid needing xlo * ylo
	xhi := x.coef / e7 // 9 digits
	xlo := x.coef % e7 // 7 digits
	yhi := y.coef / e7 // 9 digits
	ylo := y.coef % e7 // 7 digits

	c := xhi * yhi
	if (xlo | ylo) != 0 {
		c += (xlo*yhi + ylo*xhi) / e7
	}
	return New(sign, c, e-2)
}

func (x Value) Mul(y Value) Value {
	return Mul(x, y)
}

// Div returns the quotient of two Value's
func Div(x, y Value) Value {
	sign := x.sign * y.sign
	switch {
	case x.sign == signZero:
		return x
	case y.sign == signZero:
		return Inf(x.sign)
	case x.IsInf():
		if y.IsInf() {
			if sign < 0 {
				return NegOne
			}
			return One
		}
		return Inf(sign)
	case y.IsInf():
		return Zero
	}
	coef := div128(x.coef, y.coef)
	return New(sign, coef, int(x.exp)-int(y.exp))
}

func (x Value) Div(y Value) Value {
	return Div(x, y)
}

// Hash returns a hash value for a Value
func (dn Value) Hash() uint32 {
	return uint32(dn.coef>>32) ^ uint32(dn.coef) ^
		uint32(dn.sign)<<16 ^ uint32(dn.exp)<<8
}

// Format converts a number to a string with a specified format
func (dn Value) Format(mask string) string {
	if dn.IsInf() {
		return "#"
	}
	n := dn
	before := 0
	after := 0
	intpart := true
	for _, mc := range mask {
		switch mc {
		case '.':
			intpart = false
		case '#':
			if intpart {
				before++
			} else {
				after++
			}
		}
	}
	if before+after == 0 || n.Exp() > before {
		return "#" // too big to fit in mask
	}
	n = n.Round(after, HalfUp)
	e := n.Exp()
	var digits []byte
	if n.IsZero() && after == 0 {
		digits = []byte("0")
		e = 1
	} else {
		digits = strconv.AppendUint(make([]byte, 0, digitsMax), n.Coef(), 10)
		digits = bytes.TrimRight(digits, "0")
	}
	nd := len(digits)

	di := e - before
	// check(di <= 0)
	var buf strings.Builder
	sign := n.Sign()
	signok := (sign >= 0)
	frac := false
	for _, mc := range []byte(mask) {
		switch mc {
		case '#':
			if 0 <= di && di < nd {
				buf.WriteByte(digits[di])
			} else if frac || di >= 0 {
				buf.WriteByte('0')
			}
			di++
		case ',':
			if di > 0 {
				buf.WriteByte(',')
			}
		case '-', '(':
			signok = true
			if sign < 0 {
				buf.WriteByte(mc)
			}
		case ')':
			if sign < 0 {
				buf.WriteByte(mc)
			} else {
				buf.WriteByte(' ')
			}
		case '.':
			frac = true
			fallthrough
		default:
			buf.WriteByte(mc)
		}
	}
	if !signok {
		return "-" // negative not handled by mask
	}
	return buf.String()
}

func Clamp(x, min, max Value) Value {
	if x.Compare(min) < 0 {
		return min
	}
	if x.Compare(max) > 0 {
		return max
	}
	return x
}

func (x Value) Clamp(min, max Value) Value {
	if x.Compare(min) < 0 {
		return min
	}
	if x.Compare(max) > 0 {
		return max
	}
	return x
}
