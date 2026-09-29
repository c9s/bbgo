package bfxfunding

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/types"
)

var testNow = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

func trade(id int64, ago time.Duration, amount, rate float64, period int) FundingTrade {
	return FundingTrade{ID: id, Time: testNow.Add(-ago), Amount: amount, Rate: rate, Period: period}
}

func TestWeightedPercentile(t *testing.T) {
	byRate := sortedByRate([]FundingTrade{
		{Rate: 0.0003, Amount: 10},
		{Rate: 0.0001, Amount: 70},
		{Rate: 0.0002, Amount: 20},
	})

	assert.Equal(t, 0.0001, weightedPercentile(byRate, 0.5))
	assert.Equal(t, 0.0001, weightedPercentile(byRate, 0.7))
	assert.Equal(t, 0.0002, weightedPercentile(byRate, 0.8))
	assert.Equal(t, 0.0003, weightedPercentile(byRate, 0.95))
	assert.Equal(t, 0.0003, weightedPercentile(byRate, 1.0))
	assert.Equal(t, 0.0, weightedPercentile(nil, 0.5))
}

func TestDominantPeriod(t *testing.T) {
	assert.Equal(t, 30, dominantPeriod([]FundingTrade{
		{Period: 2, Amount: 100},
		{Period: 30, Amount: 80},
		{Period: 30, Amount: 80},
	}))

	// ties go to the shorter period
	assert.Equal(t, 2, dominantPeriod([]FundingTrade{{Period: 30, Amount: 10}, {Period: 2, Amount: 10}}))
	assert.Equal(t, minFundingPeriod, dominantPeriod(nil))
}

func TestTradesInWindow(t *testing.T) {
	trades := []FundingTrade{
		trade(1, 3*time.Hour, 1, 0.0001, 2),
		trade(2, 2*time.Hour, 1, 0.0001, 2),
		trade(3, time.Hour, 1, 0.0001, 2),
	}

	window := tradesInWindow(trades, testNow.Add(-2*time.Hour), testNow)
	require.Len(t, window, 2)
	assert.Equal(t, int64(2), window[0].ID)
	assert.Empty(t, tradesInWindow(trades, testNow.Add(-time.Minute), testNow))
}

func TestAnalyzeHighRateZone(t *testing.T) {
	var trades []FundingTrade
	id := int64(0)
	// 10 days of normal 2-day trades at 0.0001 and a few 30-day spike trades at 0.0005 ~ 0.0007
	for d := 10; d > 0; d-- {
		for h := range 24 {
			id++
			trades = append(trades, trade(id, time.Duration(d)*24*time.Hour-time.Duration(h)*time.Hour, 1000, 0.0001, 2))
		}

		id++
		trades = append(trades, trade(id, time.Duration(d)*24*time.Hour-30*time.Minute, 400, 0.0005, 30))
		id++
		trades = append(trades, trade(id, time.Duration(d)*24*time.Hour-20*time.Minute, 200, 0.0007, 30))
	}

	zone, err := AnalyzeHighRateZone(trades, testNow, 90*24*time.Hour, HighRateConfig{
		Percentile:     0.98,
		CeilPercentile: 1.0,
		Levels:         2,
	})
	require.NoError(t, err)

	assert.Equal(t, 0.0005, zone.Floor)
	assert.Equal(t, 0.0007, zone.Ceil)
	assert.Equal(t, 30, zone.Period)
	assert.Equal(t, []PeriodVolume{{Period: 30, Volume: 6000, Share: 1.0}}, zone.Periods)
	assert.Equal(t, 260, zone.Trades)
	assert.Equal(t, 6000.0, zone.Volume)
	assert.Equal(t, 246000.0, zone.TotalVolume)
	assert.InDelta(t, 10.0, zone.Days, 1e-9)

	// 6000 of the high rate volume over the 10 days of history (not the 90 days lookback)
	assert.InDelta(t, 600.0, zone.DailyVolume, 1.0)

	require.Len(t, zone.Levels, 2)
	assert.Equal(t, 0.0005, zone.Levels[0].Rate)
	assert.InDelta(t, 2.0/3.0, zone.Levels[0].Weight, 1e-9)
	assert.Equal(t, 0.0007, zone.Levels[1].Rate)
	assert.InDelta(t, 1.0/3.0, zone.Levels[1].Weight, 1e-9)

	_, err = AnalyzeHighRateZone(nil, testNow, time.Hour, HighRateConfig{})
	assert.ErrorIs(t, err, errNoTrades)
}

func TestAnalyzeRateChannel(t *testing.T) {
	trades := []FundingTrade{
		trade(1, 30*time.Hour, 5000, 0.0009, 2), // out of the window
		trade(2, 5*time.Hour, 100, 0.00010, 2),
		trade(3, 4*time.Hour, 100, 0.00012, 2),
		trade(4, 3*time.Hour, 100, 0.00015, 7),
		trade(5, 2*time.Hour, 100, 0.00020, 7),
		trade(6, time.Hour, 100, 0.00025, 30),
	}

	ch, err := AnalyzeRateChannel(trades, testNow, 24*time.Hour, ChannelConfig{
		Window:              types.Duration(24 * time.Hour),
		LowerPercentile:     0.2,
		UpperPercentile:     0.8,
		PeriodRateTolerance: 0.1,
	})
	require.NoError(t, err)

	assert.Equal(t, 0.00010, ch.Lower)
	assert.Equal(t, 0.00015, ch.Mid)
	assert.Equal(t, 0.00020, ch.Upper)
	assert.Equal(t, 5, ch.Trades)
	assert.Equal(t, 7, ch.Period)
	assert.Equal(t, 500.0, ch.Volume)
}

func TestRateLevels_FlatZone(t *testing.T) {
	levels := rateLevels([]FundingTrade{{Rate: 0.0005, Amount: 10}}, 0.0005, 0.0005, 3)
	assert.Equal(t, []RateLevel{{Rate: 0.0005, Weight: 1.0}}, levels)
}

func TestPeriodDistribution(t *testing.T) {
	dist := periodDistribution([]FundingTrade{
		{Period: 2, Amount: 50},
		{Period: 30, Amount: 30},
		{Period: 120, Amount: 20},
		{Period: 2, Amount: 50},
		{Period: 7, Amount: 20},
	})

	assert.Equal(t, []PeriodVolume{
		{Period: 2, Volume: 100, Share: 100.0 / 170.0},
		{Period: 30, Volume: 30, Share: 30.0 / 170.0},
		{Period: 7, Volume: 20, Share: 20.0 / 170.0},
		{Period: 120, Volume: 20, Share: 20.0 / 170.0},
	}, dist)
}
