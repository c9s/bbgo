package bfxfunding

import (
	"fmt"
	"strings"
)

// maxReportPeriods is the max number of loan periods listed in the report
const maxReportPeriods = 5

func formatPeriods(dist []PeriodVolume) string {
	var parts []string
	for i, pv := range dist {
		if i == maxReportPeriods {
			break
		}

		parts = append(parts, fmt.Sprintf("%dd %.1f%%", pv.Period, pv.Share*100))
	}

	return strings.Join(parts, ", ")
}

// logParameters logs the effective parameters after the defaults are applied
func (s *Strategy) logParameters() {
	amount := "whole funding wallet"
	if s.Amount.Sign() > 0 {
		amount = s.Amount.String()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "parameters:\n")
	fmt.Fprintf(&b, "  dryRun: %v\n", s.DryRun)
	fmt.Fprintf(&b, "  amount: %s\n", amount)
	fmt.Fprintf(&b, "  minRate: %s\n", formatRate(s.MinRate.Float64()))
	fmt.Fprintf(&b, "  lookback: %s, recomputeInterval: %s\n", s.Lookback.Duration(), s.RecomputeInterval.Duration())
	fmt.Fprintf(&b, "  offer amount: min %s, max %s\n", s.MinOfferAmount, s.MaxOfferAmount)
	fmt.Fprintf(&b, "  highRate: percentile %.3f, ceilPercentile %.3f, maxRatio %.2f, volumeShare %.3f, levels %d, rezoneTolerance %.2f\n",
		s.HighRate.Percentile, s.HighRate.CeilPercentile, s.HighRate.MaxRatio, s.HighRate.VolumeShare,
		s.HighRate.Levels, s.HighRate.RezoneTolerance)
	fmt.Fprintf(&b, "  channel: window %s, percentiles %.2f-%.2f, repriceTolerance %.2f, periodRateTolerance %.2f",
		s.Channel.Window.Duration(), s.Channel.LowerPercentile, s.Channel.UpperPercentile,
		s.Channel.RepriceTolerance, s.Channel.PeriodRateTolerance)
	s.logger.Info(b.String())
}

// logAnalysis logs the estimated rate range, periods and volumes of both tiers
func (s *Strategy) logAnalysis(zone HighRateZone, channel RateChannel) {
	var b strings.Builder
	fmt.Fprintf(&b, "funding analysis of %s:\n", s.Currency)
	fmt.Fprintf(&b, "  history: %d trades over %.1f days, total volume %.2f, avg daily volume %.2f\n",
		zone.Trades, zone.Days, zone.TotalVolume, zone.TotalVolume/zone.Days)

	fmt.Fprintf(&b, "  high rate zone (p%.1f-p%.1f): %s ~ %s\n",
		s.HighRate.Percentile*100, s.HighRate.CeilPercentile*100, formatRate(zone.Floor), formatRate(zone.Ceil))
	fmt.Fprintf(&b, "    volume %.2f (%.2f%% of total), avg daily %.2f\n",
		zone.Volume, zone.Volume/zone.TotalVolume*100, zone.DailyVolume)
	fmt.Fprintf(&b, "    period: %dd, distribution: %s\n", zone.Period, formatPeriods(zone.Periods))
	for i, l := range zone.Levels {
		fmt.Fprintf(&b, "    level #%d: %s, weight %.1f%%\n", i+1, formatRate(l.Rate), l.Weight*100)
	}

	fmt.Fprintf(&b, "  rate channel (%s, p%.0f-p%.0f): %s ~ %s, median %s\n",
		s.Channel.Window.Duration(), s.Channel.LowerPercentile*100, s.Channel.UpperPercentile*100,
		formatRate(channel.Lower), formatRate(channel.Upper), formatRate(channel.Mid))
	fmt.Fprintf(&b, "    %d trades, volume %.2f\n", channel.Trades, channel.Volume)
	fmt.Fprintf(&b, "    period at the upper edge: %dd, distribution: %s", channel.Period, formatPeriods(channel.Periods))
	s.logger.Info(b.String())
}

// allocation is how the capital is split into the tiers in a rebalance
type allocation struct {
	Capital    float64
	Idle       float64
	HighTarget float64
	KeptHigh   float64
}

// logPlan logs the capital allocation and the planned offers with their estimated yield if all get filled
func (s *Strategy) logPlan(zone HighRateZone, alloc allocation, specs []OfferSpec) {
	prefix := ""
	if s.DryRun {
		prefix = "[DRY RUN] "
	}

	byRatio := alloc.Capital * s.HighRate.MaxRatio
	byVolume := zone.DailyVolume * s.HighRate.VolumeShare
	bound := "maxRatio"
	if byVolume < byRatio {
		bound = "volumeShare"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%sallocation: capital %.8f, idle %.8f\n", prefix, alloc.Capital, alloc.Idle)
	fmt.Fprintf(&b, "  high rate target %.8f = min(capital * maxRatio = %.8f, dailyVolume * volumeShare = %.8f), bound by %s, kept %.8f\n",
		alloc.HighTarget, byRatio, byVolume, bound, alloc.KeptHigh)

	var tierAmounts = map[Tier]float64{}
	var total, weightedRate float64
	for i, spec := range specs {
		fmt.Fprintf(&b, "  offer #%d: %s\n", i+1, spec)
		tierAmounts[spec.Tier] += spec.Amount
		total += spec.Amount
		weightedRate += spec.Amount * spec.Rate
	}

	fmt.Fprintf(&b, "  planned: high %.8f, channel %.8f", tierAmounts[TierHighRate], tierAmounts[TierChannel])
	if total > 0 {
		avg := weightedRate / total
		fmt.Fprintf(&b, ", avg rate %s if all filled", formatRate(avg))
	}

	s.logger.Info(b.String())
}
