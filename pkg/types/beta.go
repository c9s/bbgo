package types

func Beta(returns, market []float64) float64 {
	if len(returns) != len(market) {
		return 0.0
	}

	returnsMean := 0.0
	marketMean := 0.0
	numDataPoints := float64(len(returns))

	for i := 0; i < len(returns); i++ {
		returnsMean += returns[i]
		marketMean += market[i]
	}

	returnsMean /= numDataPoints
	marketMean /= numDataPoints

	covar := 0.0
	marketVar := 0.0

	for i := 0; i < len(returns); i++ {
		covar += (returns[i] - returnsMean) * (market[i] - marketMean)
		marketVar += (market[i] - marketMean) * (market[i] - marketMean)
	}

	if marketVar == 0.0 {
		return 0.0 // Avoid division by zero
	}

	return covar / marketVar
}

// func MaxDrawDown(returns []float64) (max, avg float64) {
// 	maxDrawdown := math.Inf(-1)

// 	for i := 0; i < len(returns); i++ {
// 		drawdown := DrawDown(returns, i)
// 		if drawdown > maxDrawdown {
// 			maxDrawdown = drawdown
// 		}
// 	}

// 	return math.Abs(maxDrawdown)
// }

// func AverageDrawDown(returns []float64, periods int) float64 {
// 	drawdowns := make([]float64, len(returns))

// 	for i := 0; i < len(returns); i++ {
// 		drawdowns[i] = DrawDown(returns, i)
// 	}

// 	sort.Float64s(drawdowns)

// 	total_dd := math.Abs(drawdowns[0])

// 	for i := 1; i < periods; i++ {
// 		total_dd += math.Abs(drawdowns[i])
// 	}

// 	return total_dd / float64(periods)
// }

// func AverageDrawDownSquared(returns []float64, periods int) float64 {
// 	drawdowns := make([]float64, len(returns))

// 	for i := 0; i < len(returns); i++ {
// 		drawdowns[i] = math.Pow(DrawDown(returns, i), 2.0)
// 	}

// 	sort.Float64s(drawdowns)

// 	total_dd := math.Abs(drawdowns[0])

// 	for i := 1; i < periods; i++ {
// 		total_dd += math.Abs(drawdowns[i])
// 	}

// 	return total_dd / float64(periods)
// }
