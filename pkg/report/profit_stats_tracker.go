package report

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ProfitStatsTracker struct {
	types.IntervalWindow

	// Accumulated profit report
	AccumulatedProfitReport *AccumulatedProfitReport `json:"accumulatedProfitReport"`

	Market types.Market

	ProfitStatsSlice   []*types.ProfitStats
	CurrentProfitStats **types.ProfitStats

	tradeStats *types.TradeStats
}

func (p *ProfitStatsTracker) Subscribe(session *bbgo.ExchangeSession, symbol string) {
	session.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: p.Interval})
}

// InitLegacy is for backward capability. ps is the ProfitStats of the strategy, Market is the strategy Market
func (p *ProfitStatsTracker) InitLegacy(market types.Market, ps **types.ProfitStats, ts *types.TradeStats) {
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
func (p *ProfitStatsTracker) Init(market types.Market, ts *types.TradeStats) {
	ps := types.NewProfitStats(p.Market)
	p.InitLegacy(market, &ps, ts)
}

func (p *ProfitStatsTracker) Bind(session *bbgo.ExchangeSession, tradeCollector *core.TradeCollector) {
	tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
		if profit == nil {
			return
		}

		p.AddProfit(*profit)
	})

	tradeCollector.OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		p.AddTrade(trade)
	})

	// Rotate profitStats slice
	session.MarketDataStream.OnKLineClosed(types.KLineWith(p.Market.Symbol, p.Interval, func(kline types.KLine) {
		p.Rotate()
	}))
}

// Rotate the tracker to make a new ProfitStats to record the profits
func (p *ProfitStatsTracker) Rotate() {
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

func (p *ProfitStatsTracker) AddProfit(profit types.Profit) {
	(*p.CurrentProfitStats).AddProfit(profit)
}

func (p *ProfitStatsTracker) AddTrade(trade types.Trade) {
	(*p.CurrentProfitStats).AddTrade(trade)

	if p.AccumulatedProfitReport != nil {
		p.AccumulatedProfitReport.AddTrade(trade)
	}
}
