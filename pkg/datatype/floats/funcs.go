package floats

import "sort"

func Lower(arr []float64, x float64) []float64 {
	sort.Float64s(arr)

	var rst []float64
	for _, a := range arr {
		// filter prices that are Lower than the current closed price
		if a >= x {
			continue
		}

		rst = append(rst, a)
	}

	return rst
}

func Higher(arr []float64, x float64) []float64 {
	sort.Float64s(arr)

	var rst []float64
	for _, a := range arr {
		// filter prices that are Lower than the current closed price
		if a <= x {
			continue
		}
		rst = append(rst, a)
	}

	return rst
}

func Group(arr []float64, minDistance float64) []float64 {
	if len(arr) == 0 {
		return nil
	}

	var groups []float64
	var grp = []float64{arr[0]}
	for _, price := range arr {
		avg := Average(grp)
		if (price / avg) > (1.0 + minDistance) {
			groups = append(groups, avg)
			grp = []float64{price}
		} else {
			grp = append(grp, price)
		}
	}

	if len(grp) > 0 {
		groups = append(groups, Average(grp))
	}

	return groups
}

func Average(arr []float64) float64 {
	s := 0.0
	for _, a := range arr {
		s += a
	}
	return s / float64(len(arr))
}

// Multiply multiplies two float series
func Multiply(inReal0 []float64, inReal1 []float64) []float64 {
	outReal := make([]float64, len(inReal0))
	for i := 0; i < len(inReal0); i++ {
		outReal[i] = inReal0[i] * inReal1[i]
	}
	return outReal
}

// CrossOver returns true if series1 is crossing over series2.
//
//    NOTE: Usually this is used with Media Average Series to check if it crosses for buy signals.
//          It assumes first values are the most recent.
//          The crossover function does not use most recent value, since usually it's not a complete candle.
//          The second recent values and the previous are used, instead.
//
// ported from https://github.com/markcheno/go-talib/blob/master/talib.go
func CrossOver(series1 []float64, series2 []float64) bool {
	if len(series1) < 3 || len(series2) < 3 {
		return false
	}

	N := len(series1)

	return series1[N-2] <= series2[N-2] && series1[N-1] > series2[N-1]
}

// CrossUnder returns true if series1 is crossing under series2.
//
//    NOTE: Usually this is used with Media Average Series to check if it crosses for sell signals.
//
// ported from https://github.com/markcheno/go-talib/blob/master/talib.go
func CrossUnder(series1 []float64, series2 []float64) bool {
	if len(series1) < 3 || len(series2) < 3 {
		return false
	}

	N := len(series1)

	return series1[N-1] <= series2[N-1] && series1[N-2] > series2[N-2]
}

// MinMax - Lowest and highest values over a specified period
// ported from https://github.com/markcheno/go-talib/blob/master/talib.go
func MinMax(inReal []float64, inTimePeriod int) (outMin []float64, outMax []float64) {
	outMin = make([]float64, len(inReal))
	outMax = make([]float64, len(inReal))
	nbInitialElementNeeded := inTimePeriod - 1
	startIdx := nbInitialElementNeeded
	outIdx := startIdx
	today := startIdx
	trailingIdx := startIdx - nbInitialElementNeeded
	highestIdx := -1
	highest := 0.0
	lowestIdx := -1
	lowest := 0.0
	for today < len(inReal) {
		tmpLow, tmpHigh := inReal[today], inReal[today]
		if highestIdx < trailingIdx {
			highestIdx = trailingIdx
			highest = inReal[highestIdx]
			i := highestIdx
			i++
			for i <= today {
				tmpHigh = inReal[i]
				if tmpHigh > highest {
					highestIdx = i
					highest = tmpHigh
				}
				i++
			}
		} else if tmpHigh >= highest {
			highestIdx = today
			highest = tmpHigh
		}
		if lowestIdx < trailingIdx {
			lowestIdx = trailingIdx
			lowest = inReal[lowestIdx]
			i := lowestIdx
			i++
			for i <= today {
				tmpLow = inReal[i]
				if tmpLow < lowest {
					lowestIdx = i
					lowest = tmpLow
				}
				i++
			}
		} else if tmpLow <= lowest {
			lowestIdx = today
			lowest = tmpLow
		}
		outMax[outIdx] = highest
		outMin[outIdx] = lowest
		outIdx++
		trailingIdx++
		today++
	}
	return outMin, outMax
}
