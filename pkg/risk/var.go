package risk

import (
	"math/rand"
	"sort"
)

func SimulateLoss(start_price float64, rnd *rand.Rand, price_func func(float64) float64) float64 {
	return start_price - price_func(rnd.NormFloat64())
}

func NewModelVaRMonteCarlo(start_price float64, price_func func(float64) float64, seed int64, n_sims int) []float64 {
	losses := make([]float64, n_sims)
	source := rand.NewSource(seed)
	rnd := rand.New(source)
	for i := range n_sims {
		losses[i] = SimulateLoss(start_price, rnd, price_func)
	}
	sort.Float64s(losses)
	return losses

}

func ComputeVaRMonteCarlo(start_price float64, price_func func(float64) float64, confidence float64, seed int64, n_sims int) float64 {
	losses := NewModelVaRMonteCarlo(start_price, price_func, seed, n_sims)
	idx := int((1.0 - confidence) * float64(n_sims))
	return losses[idx]
}

func ComputeExpectedShortfall(start_price float64, price_func func(float64) float64, confidence float64, seed int64, n_sims int) float64 {
	idx := int((1.0 - confidence) * float64(n_sims))
	losses := NewModelVaRMonteCarlo(start_price, price_func, seed, n_sims)
	result := 0.0
	for i := range idx {
		result += losses[i]
	}
	return result / float64(idx)
}
