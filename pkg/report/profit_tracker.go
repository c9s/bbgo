package report

import (
	"github.com/c9s/bbgo/pkg/types"
)

type ProfitTracker struct {
	types.IntervalWindow

	// Accumulated profit report
	AccumulatedProfitReport *AccumulatedProfitReport `json:"accumulatedProfitReport"`

	Market types.Market

	ProfitStatsSlice   []*types.ProfitStats
	CurrentProfitStats **types.ProfitStats

	tradeStats *types.TradeStats
}

// InitOld is for backward capability. ps is the ProfitStats of the strategy, Market is the strategy Market
func (p *ProfitTracker) InitOld(market types.Market, ps **types.ProfitStats, ts *types.TradeStats) {
	p.Market = market

	if *ps == nil {
		*ps = types.NewProfitStats(p.Market)
	}

	p.tradeStats = ts

	p.CurrentProfitStats = ps
	p.ProfitStatsSlice = append(p.ProfitStatsSlice, *ps)

	if p.AccumulatedProfitReport != nil {
		p.AccumulatedProfitReport.Initialize(p.Market.Symbol, p.Interval, p.Window)
	}
}

// Init initialize the tracker with the given Market
func (p *ProfitTracker) Init(market types.Market, ts *types.TradeStats) {
	ps := types.NewProfitStats(p.Market)
	p.InitOld(market, &ps, ts)
}

// Rotate the tracker to make a new ProfitStats to record the profits
func (p *ProfitTracker) Rotate() {
	// Update report
	if p.AccumulatedProfitReport != nil {
		p.AccumulatedProfitReport.Rotate(*p.CurrentProfitStats, p.tradeStats)
	}

	*p.CurrentProfitStats = types.NewProfitStats(p.Market)
	p.ProfitStatsSlice = append(p.ProfitStatsSlice, *p.CurrentProfitStats)
	// Truncate
	if len(p.ProfitStatsSlice) > p.Window {
		p.ProfitStatsSlice = p.ProfitStatsSlice[len(p.ProfitStatsSlice)-p.Window:]
	}
}

func (p *ProfitTracker) AddProfit(profit types.Profit) {
	(*p.CurrentProfitStats).AddProfit(profit)
}

func (p *ProfitTracker) AddTrade(trade types.Trade) {
	(*p.CurrentProfitStats).AddTrade(trade)

	if p.AccumulatedProfitReport != nil {
		p.AccumulatedProfitReport.AddTrade(trade)
	}
}
