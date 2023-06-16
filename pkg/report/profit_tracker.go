package report

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ProfitTracker struct {
	types.IntervalWindow

	market             types.Market
	profitStatsSlice   []*types.ProfitStats
	currentProfitStats **types.ProfitStats
}

// InitOld is for backward capability. ps is the ProfitStats of the strategy, market is the strategy market
func (p *ProfitTracker) InitOld(ps **types.ProfitStats, market types.Market) {
	p.market = market

	if *ps == nil {
		*ps = types.NewProfitStats(p.market)
	}

	p.currentProfitStats = ps
	p.profitStatsSlice = append(p.profitStatsSlice, *ps)
}

// Init initialize the tracker with the given market
func (p *ProfitTracker) Init(market types.Market) {
	p.market = market
	*p.currentProfitStats = types.NewProfitStats(p.market)
	p.profitStatsSlice = append(p.profitStatsSlice, *p.currentProfitStats)
}

func (p *ProfitTracker) Bind(tradeCollector *bbgo.TradeCollector, session *bbgo.ExchangeSession) {
	// TODO: Register kline close callback
	tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
		p.AddProfit(*profit)
	})

	tradeCollector.OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {

	})

	session.MarketDataStream.OnKLineClosed(types.KLineWith(p.market.Symbol, p.Interval, func(kline types.KLine) {

	}))
}

// Rotate the tracker to make a new ProfitStats to record the profits
func (p *ProfitTracker) Rotate() {
	*p.currentProfitStats = types.NewProfitStats(p.market)
	p.profitStatsSlice = append(p.profitStatsSlice, *p.currentProfitStats)
	// Truncate
	if len(p.profitStatsSlice) > p.Window {
		p.profitStatsSlice = p.profitStatsSlice[len(p.profitStatsSlice)-p.Window:]
	}
}

func (p *ProfitTracker) AddProfit(profit types.Profit) {
	(*p.currentProfitStats).AddProfit(profit)
}
