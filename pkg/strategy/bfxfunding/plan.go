package bfxfunding

import (
	"fmt"
	"math"
)

type Tier string

const (
	TierHighRate Tier = "high"
	TierChannel  Tier = "channel"
)

// OfferSpec is a funding offer to be submitted
type OfferSpec struct {
	Tier   Tier
	Rate   float64
	Period int
	Amount float64
}

func (o OfferSpec) String() string {
	return fmt.Sprintf("%s offer %.8f @ %s for %dd", o.Tier, o.Amount, formatRate(o.Rate), o.Period)
}

// HighRateTarget returns the capital that the high rate tier should hold:
// min(capital * maxRatio, avgDailyHighVolume * volumeShare)
func HighRateTarget(capital float64, zone HighRateZone, cfg HighRateConfig) float64 {
	return math.Max(0, math.Min(capital*cfg.MaxRatio, zone.DailyVolume*cfg.VolumeShare))
}

// OfferLimits are the constraints applied to every planned offer
type OfferLimits struct {
	MinAmount float64
	MaxAmount float64
	MinRate   float64
}

// PlanHighRateOffers spreads the amount across the zone levels by their volume weights.
// The levels whose share is below the min offer amount are dropped and their share goes to the other levels.
func PlanHighRateOffers(amount float64, zone HighRateZone, limits OfferLimits) []OfferSpec {
	if amount < limits.MinAmount || len(zone.Levels) == 0 {
		return nil
	}

	levels := zone.Levels
	for {
		var totalWeight float64
		for _, l := range levels {
			totalWeight += l.Weight
		}

		var kept []RateLevel
		for _, l := range levels {
			if amount*l.Weight/totalWeight >= limits.MinAmount {
				kept = append(kept, l)
			}
		}

		if len(kept) == len(levels) {
			break
		}

		if len(kept) == 0 {
			// not enough amount to spread, put everything on the heaviest level
			heaviest := levels[0]
			for _, l := range levels {
				if l.Weight > heaviest.Weight {
					heaviest = l
				}
			}

			kept = []RateLevel{heaviest}
		}

		levels = kept
	}

	var totalWeight float64
	for _, l := range levels {
		totalWeight += l.Weight
	}

	var offers []OfferSpec
	for _, l := range levels {
		rate := math.Max(l.Rate, limits.MinRate)
		for _, a := range splitAmount(amount*l.Weight/totalWeight, limits) {
			offers = append(offers, OfferSpec{Tier: TierHighRate, Rate: rate, Period: zone.Period, Amount: a})
		}
	}

	return offers
}

// PlanChannelOffers places the amount at the upper edge of the rate channel
func PlanChannelOffers(amount float64, ch RateChannel, limits OfferLimits) []OfferSpec {
	rate := math.Max(ch.Upper, limits.MinRate)

	var offers []OfferSpec
	for _, a := range splitAmount(amount, limits) {
		offers = append(offers, OfferSpec{Tier: TierChannel, Rate: rate, Period: ch.Period, Amount: a})
	}

	return offers
}

// splitAmount splits the amount into equal chunks of at most MaxAmount.
// MinAmount is the exchange requirement so it wins over MaxAmount when both can't be satisfied,
// and an amount below MinAmount is not placed at all.
func splitAmount(amount float64, limits OfferLimits) []float64 {
	if amount <= 0 || amount < limits.MinAmount {
		return nil
	}

	n := 1
	if limits.MaxAmount > 0 {
		n = int(math.Ceil(amount / limits.MaxAmount))
	}

	if limits.MinAmount > 0 && amount/float64(n) < limits.MinAmount {
		n = int(math.Floor(amount / limits.MinAmount))
	}

	chunk := amount / float64(n)
	chunks := make([]float64, n)
	for i := range chunks {
		chunks[i] = chunk
	}

	return chunks
}
