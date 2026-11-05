package bfxfunding

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

const (
	minFundingPeriod = 2
	maxFundingPeriod = 120
)

var errNoTrades = errors.New("no funding trades in the analysis window")

// HighRateConfig configures the tier that parks capital in the high rate zone
type HighRateConfig struct {
	// Percentile is the volume-weighted rate percentile over the lookback that defines the floor of the high rate zone
	Percentile float64 `json:"percentile"`

	// CeilPercentile is the volume-weighted rate percentile that defines the ceil of the high rate zone,
	// the rates above it are considered outliers.
	CeilPercentile float64 `json:"ceilPercentile"`

	// MaxRatio caps the ratio of the capital allocated to the high rate tier
	MaxRatio float64 `json:"maxRatio"`

	// VolumeShare is the share of the average daily volume filled in the high rate zone that the tier expects to take,
	// the tier amount is min(capital * maxRatio, avgDailyHighVolume * volumeShare)
	VolumeShare float64 `json:"volumeShare"`

	// Levels is the number of rate levels the tier amount is spread across the zone
	Levels int `json:"levels"`

	// RezoneTolerance is the relative change of the zone floor that triggers re-placing the tier offers
	RezoneTolerance float64 `json:"rezoneTolerance"`
}

// ChannelConfig configures the tier that places capital at the upper edge of the normal rate channel
type ChannelConfig struct {
	// Window is the rolling window of the rate channel
	Window types.Duration `json:"window"`

	LowerPercentile float64 `json:"lowerPercentile"`
	UpperPercentile float64 `json:"upperPercentile"`

	// RepriceTolerance is the relative rate drift of an offer from the channel edge that triggers re-placing it
	RepriceTolerance float64 `json:"repriceTolerance"`

	// PeriodRateTolerance is the relative rate range around the channel edge used to find the dominant period
	PeriodRateTolerance float64 `json:"periodRateTolerance"`
}

// PeriodVolume is the volume filled with a loan period and its share of the total volume
type PeriodVolume struct {
	Period int
	Volume float64
	Share  float64
}

// RateLevel is a rate in the high rate zone with its share of the zone volume
type RateLevel struct {
	Rate   float64
	Weight float64
}

type HighRateZone struct {
	Floor float64
	Ceil  float64

	// Period is the loan period (days) with the most volume in the zone
	Period int

	// DailyVolume is the average daily volume filled at or above the floor
	DailyVolume float64

	// Volume is the volume filled at or above the floor, TotalVolume is the volume of all the trades in the lookback
	Volume      float64
	TotalVolume float64

	// Trades is the number of the trades in the lookback, Days is the time span they cover
	Trades int
	Days   float64

	// Periods is the volume distribution of the loan periods in the zone
	Periods []PeriodVolume

	Levels []RateLevel
}

func (z HighRateZone) String() string {
	return fmt.Sprintf("HighRateZone{floor: %s, ceil: %s, period: %dd, dailyVolume: %.2f, levels: %d}",
		formatRate(z.Floor), formatRate(z.Ceil), z.Period, z.DailyVolume, len(z.Levels))
}

type RateChannel struct {
	Lower float64
	Mid   float64
	Upper float64

	// Period is the loan period (days) with the most volume around the upper edge
	Period int

	Volume float64
	Trades int

	// Periods is the volume distribution of the loan periods around the upper edge
	Periods []PeriodVolume
}

func (c RateChannel) String() string {
	return fmt.Sprintf("RateChannel{lower: %s, upper: %s, period: %dd, volume: %.2f}",
		formatRate(c.Lower), formatRate(c.Upper), c.Period, c.Volume)
}

// AnalyzeHighRateZone finds the high rate zone from the trades in [now - lookback, now]
func AnalyzeHighRateZone(trades []FundingTrade, now time.Time, lookback time.Duration, cfg HighRateConfig) (HighRateZone, error) {
	window := tradesInWindow(trades, now.Add(-lookback), now)
	if len(window) == 0 {
		return HighRateZone{}, errNoTrades
	}

	byRate := sortedByRate(window)
	zone := HighRateZone{
		Floor: weightedPercentile(byRate, cfg.Percentile),
		Ceil:  weightedPercentile(byRate, cfg.CeilPercentile),
	}

	var highTrades []FundingTrade
	for _, t := range byRate {
		zone.TotalVolume += t.Amount
		if t.Rate >= zone.Floor {
			highTrades = append(highTrades, t)
			zone.Volume += t.Amount
		}
	}

	// the history could be shorter than the lookback while it's being backfilled
	zone.Trades = len(window)
	zone.Days = math.Max(now.Sub(window[0].Time).Hours()/24.0, 1.0)
	zone.DailyVolume = zone.Volume / zone.Days
	zone.Period = dominantPeriod(highTrades)
	zone.Periods = periodDistribution(highTrades)
	zone.Levels = rateLevels(highTrades, zone.Floor, zone.Ceil, cfg.Levels)
	return zone, nil
}

// AnalyzeRateChannel finds the normal rate channel from the trades in [now - window, now]
func AnalyzeRateChannel(trades []FundingTrade, now time.Time, window time.Duration, cfg ChannelConfig) (RateChannel, error) {
	inWindow := tradesInWindow(trades, now.Add(-window), now)
	if len(inWindow) == 0 {
		return RateChannel{}, errNoTrades
	}

	byRate := sortedByRate(inWindow)
	ch := RateChannel{
		Lower:  weightedPercentile(byRate, cfg.LowerPercentile),
		Mid:    weightedPercentile(byRate, 0.5),
		Upper:  weightedPercentile(byRate, cfg.UpperPercentile),
		Trades: len(inWindow),
	}

	var edgeTrades []FundingTrade
	for _, t := range byRate {
		ch.Volume += t.Amount
		if math.Abs(t.Rate-ch.Upper) <= ch.Upper*cfg.PeriodRateTolerance {
			edgeTrades = append(edgeTrades, t)
		}
	}

	if len(edgeTrades) == 0 {
		edgeTrades = byRate
	}

	ch.Period = dominantPeriod(edgeTrades)
	ch.Periods = periodDistribution(edgeTrades)

	return ch, nil
}

// tradesInWindow returns the trades in [from, to], the trades must be sorted by time
func tradesInWindow(trades []FundingTrade, from, to time.Time) []FundingTrade {
	i := sort.Search(len(trades), func(i int) bool { return !trades[i].Time.Before(from) })
	j := sort.Search(len(trades), func(i int) bool { return trades[i].Time.After(to) })
	if i >= j {
		return nil
	}

	return trades[i:j]
}

func sortedByRate(trades []FundingTrade) []FundingTrade {
	sorted := make([]FundingTrade, len(trades))
	copy(sorted, trades)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Rate < sorted[j].Rate })
	return sorted
}

// weightedPercentile returns the rate at which the cumulative volume reaches p of the total volume,
// the trades must be sorted by rate.
func weightedPercentile(byRate []FundingTrade, p float64) float64 {
	if len(byRate) == 0 {
		return 0
	}

	var total float64
	for _, t := range byRate {
		total += t.Amount
	}

	target := total * p
	var cum float64
	for _, t := range byRate {
		cum += t.Amount
		if cum >= target {
			return t.Rate
		}
	}

	return byRate[len(byRate)-1].Rate
}

// dominantPeriod returns the loan period with the most volume, falls back to the min period if there is no trade
func dominantPeriod(trades []FundingTrade) int {
	volumes := make(map[int]float64)
	for _, t := range trades {
		volumes[t.Period] += t.Amount
	}

	period, maxVolume := minFundingPeriod, 0.0
	for p, v := range volumes {
		// prefer the shorter period when the volumes are equal to make the result deterministic
		if v > maxVolume || (v == maxVolume && p < period) {
			period, maxVolume = p, v
		}
	}

	return clampPeriod(period)
}

// periodDistribution returns the volume of each loan period, sorted by the volume in descending order
func periodDistribution(trades []FundingTrade) []PeriodVolume {
	volumes := make(map[int]float64)
	var total float64
	for _, t := range trades {
		volumes[t.Period] += t.Amount
		total += t.Amount
	}

	dist := make([]PeriodVolume, 0, len(volumes))
	for p, v := range volumes {
		dist = append(dist, PeriodVolume{Period: p, Volume: v, Share: v / total})
	}

	sort.Slice(dist, func(i, j int) bool {
		if dist[i].Volume == dist[j].Volume {
			return dist[i].Period < dist[j].Period
		}

		return dist[i].Volume > dist[j].Volume
	})

	return dist
}

// rateLevels splits [floor, ceil] into n buckets and returns the volume-weighted median rate of each bucket
// with its share of the zone volume, so that the offers are placed at the rates where the trades were filled.
func rateLevels(trades []FundingTrade, floor, ceil float64, n int) []RateLevel {
	if n <= 0 {
		n = 1
	}

	width := (ceil - floor) / float64(n)
	if width <= 0 {
		return []RateLevel{{Rate: floor, Weight: 1.0}}
	}

	buckets := make([][]FundingTrade, n)
	var total float64
	for _, t := range trades {
		if t.Rate < floor || t.Rate > ceil {
			continue
		}

		i := min(int((t.Rate-floor)/width), n-1)
		buckets[i] = append(buckets[i], t)
		total += t.Amount
	}

	if total == 0 {
		return []RateLevel{{Rate: floor, Weight: 1.0}}
	}

	var levels []RateLevel
	for _, bucket := range buckets {
		if len(bucket) == 0 {
			continue
		}

		var volume float64
		for _, t := range bucket {
			volume += t.Amount
		}

		levels = append(levels, RateLevel{
			Rate:   weightedPercentile(sortedByRate(bucket), 0.5),
			Weight: volume / total,
		})
	}

	return levels
}

func clampPeriod(p int) int {
	return max(minFundingPeriod, min(p, maxFundingPeriod))
}

func formatRate(r float64) string {
	return fmt.Sprintf("%.8f (%.2f%% APR)", r, r*365*100)
}
