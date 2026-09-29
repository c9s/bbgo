package bfxfunding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHighRateTarget(t *testing.T) {
	cfg := HighRateConfig{MaxRatio: 0.4, VolumeShare: 0.05}

	// capped by the volume share: 100000 * 0.05 = 5000 < 50000 * 0.4
	assert.Equal(t, 5000.0, HighRateTarget(50000, HighRateZone{DailyVolume: 100000}, cfg))

	// capped by the max ratio: 10000 * 0.4 = 4000 < 1000000 * 0.05
	assert.Equal(t, 4000.0, HighRateTarget(10000, HighRateZone{DailyVolume: 1000000}, cfg))
}

func TestSplitAmount(t *testing.T) {
	limits := OfferLimits{MinAmount: 150, MaxAmount: 1000}

	assert.Nil(t, splitAmount(100, limits))
	assert.Equal(t, []float64{500}, splitAmount(500, limits))
	assert.Equal(t, []float64{833.3333333333334, 833.3333333333334, 833.3333333333334}, splitAmount(2500, limits))

	// the min amount wins over the max amount
	assert.Equal(t, []float64{160}, splitAmount(160, OfferLimits{MinAmount: 150, MaxAmount: 150}))

	// no split without max amount
	assert.Equal(t, []float64{2500}, splitAmount(2500, OfferLimits{MinAmount: 150}))
}

func TestPlanHighRateOffers(t *testing.T) {
	zone := HighRateZone{
		Period: 30,
		Levels: []RateLevel{
			{Rate: 0.0005, Weight: 0.6},
			{Rate: 0.0006, Weight: 0.3},
			{Rate: 0.0007, Weight: 0.1},
		},
	}

	t.Run("spread by weights", func(t *testing.T) {
		offers := PlanHighRateOffers(3000, zone, OfferLimits{MinAmount: 150})
		require.Len(t, offers, 3)
		assert.InDelta(t, 1800.0, offers[0].Amount, 1e-6)
		assert.InDelta(t, 900.0, offers[1].Amount, 1e-6)
		assert.InDelta(t, 300.0, offers[2].Amount, 1e-6)
		for _, o := range offers {
			assert.Equal(t, TierHighRate, o.Tier)
			assert.Equal(t, 30, o.Period)
		}
	})

	t.Run("drop the levels below the min amount", func(t *testing.T) {
		// 1000 * 0.1 = 100 < 150, the level is dropped and its share goes to the others
		offers := PlanHighRateOffers(1000, zone, OfferLimits{MinAmount: 150})
		require.Len(t, offers, 2)
		assert.InDelta(t, 666.666666, offers[0].Amount, 1e-5)
		assert.InDelta(t, 333.333333, offers[1].Amount, 1e-5)
	})

	t.Run("put everything on the heaviest level", func(t *testing.T) {
		offers := PlanHighRateOffers(200, zone, OfferLimits{MinAmount: 150})
		require.Len(t, offers, 1)
		assert.Equal(t, 0.0005, offers[0].Rate)
		assert.Equal(t, 200.0, offers[0].Amount)
	})

	t.Run("min rate floor", func(t *testing.T) {
		offers := PlanHighRateOffers(200, zone, OfferLimits{MinAmount: 150, MinRate: 0.001})
		require.Len(t, offers, 1)
		assert.Equal(t, 0.001, offers[0].Rate)
	})

	t.Run("below min amount", func(t *testing.T) {
		assert.Empty(t, PlanHighRateOffers(100, zone, OfferLimits{MinAmount: 150}))
	})
}

func TestPlanChannelOffers(t *testing.T) {
	ch := RateChannel{Lower: 0.0001, Upper: 0.0002, Period: 2}

	offers := PlanChannelOffers(2500, ch, OfferLimits{MinAmount: 150, MaxAmount: 1000})
	require.Len(t, offers, 3)
	for _, o := range offers {
		assert.Equal(t, TierChannel, o.Tier)
		assert.Equal(t, 0.0002, o.Rate)
		assert.Equal(t, 2, o.Period)
	}

	offers = PlanChannelOffers(500, ch, OfferLimits{MinAmount: 150, MinRate: 0.0003})
	require.Len(t, offers, 1)
	assert.Equal(t, 0.0003, offers[0].Rate)
}
