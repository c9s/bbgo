package pivotshort

import "sort"

func lower(arr []float64, x float64) []float64 {
	sort.Float64s(arr)

	var rst []float64
	for _, a := range arr {
		// filter prices that are lower than the current closed price
		if a > x {
			continue
		}

		rst = append(rst, a)
	}

	return rst
}

func higher(arr []float64, x float64) []float64 {
	sort.Float64s(arr)

	var rst []float64
	for _, a := range arr {
		// filter prices that are lower than the current closed price
		if a < x {
			continue
		}
		rst = append(rst, a)
	}

	return rst
}

func group(arr []float64, minDistance float64) []float64 {
	if len(arr) == 0 {
		return nil
	}

	var groups []float64
	var grp = []float64{arr[0]}
	for _, price := range arr {
		avg := average(grp)
		if (price / avg) > (1.0 + minDistance) {
			groups = append(groups, avg)
			grp = []float64{price}
		} else {
			grp = append(grp, price)
		}
	}

	if len(grp) > 0 {
		groups = append(groups, average(grp))
	}

	return groups
}

func average(arr []float64) float64 {
	s := 0.0
	for _, a := range arr {
		s += a
	}
	return s / float64(len(arr))
}
