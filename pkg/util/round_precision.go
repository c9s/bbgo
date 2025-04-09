package util

import (
	"math"
	"strconv"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// RoundAndTruncatePrice rounds the given price at prec-1 and then truncate the price at prec
func RoundAndTruncatePrice(p fixedpoint.Value, prec int) fixedpoint.Value {
	var pow10 = math.Pow10(prec)
	pp := math.Round(p.Float64()*pow10*10.0) / 10.0
	pp = math.Trunc(pp) / pow10

	pps := strconv.FormatFloat(pp, 'f', prec, 64)
	price := fixedpoint.MustNewFromString(pps)
	return price
}
