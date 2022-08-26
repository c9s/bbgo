package util

import (
	"math"
	"strconv"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

const MaxDigits = 18 // MAX_INT64 ~ 9 * 10^18

var Pow10Table = [MaxDigits + 1]int64{
	1, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18,
}

func Pow10(n int64) int64 {
	if n < 0 || n > MaxDigits {
		return 0
	}
	return Pow10Table[n]
}

func FormatValue(val fixedpoint.Value, prec int) string {
	return val.FormatString(prec)
}

func FormatFloat(val float64, prec int) string {
	return strconv.FormatFloat(val, 'f', prec, 64)
}

func ParseFloat(s string) (float64, error) {
	if len(s) == 0 {
		return 0.0, nil
	}

	return strconv.ParseFloat(s, 64)
}

func MustParseFloat(s string) float64 {
	if len(s) == 0 {
		return 0.0
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(err)
	}
	return v
}

const epsilon = 0.0000001

func Zero(v float64) bool {
	return math.Abs(v) < epsilon
}

func NotZero(v float64) bool {
	return math.Abs(v) > epsilon
}

