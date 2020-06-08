package util

import (
	"math"
	"strconv"
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

var NegPow10Table = [MaxDigits + 1]float64{
	1, 1e-1, 1e-2, 1e-3, 1e-4, 1e-5, 1e-6, 1e-7, 1e-8, 1e-9, 1e-10, 1e-11, 1e-12, 1e-13, 1e-14, 1e-15, 1e-16, 1e-17, 1e-18,
}

func NegPow10(n int64) float64 {
	if n < 0 || n > MaxDigits {
		return 0.0
	}
	return NegPow10Table[n]
}

func Float64ToStr(input float64) string {
	return strconv.FormatFloat(input, 'f', -1, 64)
}

func Float64ToInt64(input float64) int64 {
	// eliminate rounding error for IEEE754 floating points
	return int64(math.Round(input))
}
