package report

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/types"
)

type TradeAdder interface {
	AddTrade(trade *types.Trade)
}

type ProfitAdder interface {
	AddProfit(trade *types.Profit)
}

// StatsCollector is the v2 profit stats tracker
type StatsCollector struct {
	Market   types.Market   `json:"market"`
	Interval types.Interval `json:"interval"`
	Window   int            `json:"window"`

	CurrentProfitStats     *types.ProfitStats  `json:"profitStats"`
	AccumulatedProfitStats *types.ProfitStats  `json:"accumulatedProfitStats"`
	HistoryProfitStats     []types.ProfitStats `json:"historyProfitStats"`

	CurrentTradeStats     *types.TradeStats  `json:"tradeStats"`
	AccumulatedTradeStats *types.TradeStats  `json:"accumulatedTradeStats"`
	HistoryTradeStats     []types.TradeStats `json:"historyTradeStats"`

	tradeCollector *core.TradeCollector
}

func NewStatsCollector(market types.Market, interval types.Interval, window int, tradeCollector *core.TradeCollector) *StatsCollector {
	return &StatsCollector{
		Market:                 market,
		Interval:               interval,
		Window:                 window,
		CurrentProfitStats:     types.NewProfitStats(market),
		CurrentTradeStats:      types.NewTradeStats(market.Symbol),
		AccumulatedProfitStats: types.NewProfitStats(market),
		AccumulatedTradeStats:  types.NewTradeStats(market.Symbol),
		tradeCollector:         tradeCollector,
	}
}

func (c *StatsCollector) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, c.Market.Symbol, types.SubscribeOptions{Interval: c.Interval})
}

func (c *StatsCollector) Bind(session *bbgo.ExchangeSession) {
	c.tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
		if profit != nil {
			c.CurrentProfitStats.AddProfit(profit)
			c.AccumulatedProfitStats.AddProfit(profit)
		}

		c.CurrentProfitStats.AddTrade(trade)
		c.AccumulatedProfitStats.AddTrade(trade)

		c.CurrentTradeStats.AddProfit(profit)
		c.AccumulatedTradeStats.AddProfit(profit)
	})

	// Rotate profitStats slice
	session.MarketDataStream.OnKLineClosed(types.KLineWith(c.Market.Symbol, c.Interval, func(k types.KLine) {
		// p.Rotate()
	}))
}

// Rotate the tracker to make a new ProfitStats to record the profits
func (c *StatsCollector) Rotate() {
	c.HistoryProfitStats = append(c.HistoryProfitStats, *c.CurrentProfitStats)
	c.HistoryTradeStats = append(c.HistoryTradeStats, *c.CurrentTradeStats)
	/*
		*p.CurrentProfitStats = types.NewProfitStats(p.Market)
		p.ProfitStatsSlice = append(p.ProfitStatsSlice, *p.CurrentProfitStats)
		// Truncate
		if len(p.ProfitStatsSlice) > p.Window {
			p.ProfitStatsSlice = p.ProfitStatsSlice[len(p.ProfitStatsSlice)-p.Window:]
		}
	*/
}
