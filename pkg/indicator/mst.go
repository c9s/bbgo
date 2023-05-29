package indicator

import "github.com/c9s/bbgo/pkg/datatype/floats"

func mst(values floats.Slice) float64 {
	var sumX, sumY, sumXSqr, sumXY = .0, .0, .0, .0

	end := len(values) - 1
	for i := end; i >= 0; i-- {
		val := values[i]
		per := float64(end - i + 1)
		sumX += per
		sumY += val
		sumXSqr += per * per
		sumXY += val * per
	}

	length := float64(len(values))
	slope := (length*sumXY - sumX*sumY) / (length*sumXSqr - sumX*sumX)

	average := sumY / length
	tail := average - slope*sumX/length + slope
	head := tail + slope*(length-1)
	slope2 := (tail - head) / (length - 1)
	return slope2
}
