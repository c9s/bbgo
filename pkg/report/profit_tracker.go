package report

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ProfitTracker struct {
	types.IntervalWindow

	ProfitStatsSlice   []*types.ProfitStats
	CurrentProfitStats **types.ProfitStats

	market types.Market
}

// InitOld is for backward capability. ps is the ProfitStats of the strategy, market is the strategy market
func (p *ProfitTracker) InitOld(ps **types.ProfitStats, market types.Market) {
	p.market = market

	if *ps == nil {
		*ps = types.NewProfitStats(p.market)
	}

	p.CurrentProfitStats = ps
	p.ProfitStatsSlice = append(p.ProfitStatsSlice, *ps)
}

// Init initialize the tracker with the given market
func (p *ProfitTracker) Init(market types.Market) {
	p.market = market
	*p.CurrentProfitStats = types.NewProfitStats(p.market)
	p.ProfitStatsSlice = append(p.ProfitStatsSlice, *p.CurrentProfitStats)
}

func (p *ProfitTracker) Bind(tradeCollector *bbgo.TradeCollector, session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, p.market.Symbol, types.SubscribeOptions{Interval: p.Interval})

	tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
		p.AddProfit(*profit)
	})

	tradeCollector.OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		p.AddTrade(trade)
	})

	// Rotate profitStats slice
	session.MarketDataStream.OnKLineClosed(types.KLineWith(p.market.Symbol, p.Interval, func(kline types.KLine) {
		p.Rotate()
	}))
}

// Rotate the tracker to make a new ProfitStats to record the profits
func (p *ProfitTracker) Rotate() {
	*p.CurrentProfitStats = types.NewProfitStats(p.market)
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
}
