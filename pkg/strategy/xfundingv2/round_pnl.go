package xfundingv2

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/style"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/slack-go/slack"
)

type RoundPnL struct {
	FundingIncome      fixedpoint.Value
	SpotProfitStats    *types.ProfitStats
	FuturesProfitStats *types.ProfitStats
}

func (p *RoundPnL) SlackAttachment() slack.Attachment {
	totalNetPnL := p.FundingIncome.Add(
		p.SpotProfitStats.AccumulatedNetProfit,
	).Add(
		p.FuturesProfitStats.AccumulatedNetProfit,
	)
	color := style.PnLColor(totalNetPnL)
	return slack.Attachment{
		Title: fmt.Sprintf("Arbitrage Round PnL: %s", p.SpotProfitStats.Symbol),
		Color: color,
		Fields: []slack.AttachmentField{
			{Title: "Funding Income", Value: p.FundingIncome.String(), Short: true},
			{Title: "Spot PnL", Value: p.SpotProfitStats.AccumulatedPnL.String(), Short: true},
			{Title: "Spot Net PnL", Value: p.SpotProfitStats.AccumulatedNetProfit.String(), Short: true},
			{Title: "Futures PnL", Value: p.FuturesProfitStats.AccumulatedPnL.String(), Short: true},
			{Title: "Futures Net PnL", Value: p.FuturesProfitStats.AccumulatedNetProfit.String(), Short: true},
			{Title: "Total Net PnL", Value: totalNetPnL.String(), Short: true},
		},
	}
}

func (p *RoundPnL) NetPnL() fixedpoint.Value {
	return p.FundingIncome.Add(
		p.SpotProfitStats.AccumulatedNetProfit,
	).Add(
		p.FuturesProfitStats.AccumulatedNetProfit,
	)
}

func (r *ArbitrageRound) PnL() RoundPnL {
	r.mu.Lock()
	defer r.mu.Unlock()

	fundingIncome := r.totalFundingIncome()

	spotMarket := r.spotWorker.Market()
	futuresMarket := r.futuresWorker.Market()

	spotPosition := types.NewPositionFromMarket(spotMarket)
	futuresPosition := types.NewPositionFromMarket(futuresMarket)
	if r.spotExchangeFeeRates != nil {
		spotPosition.ExchangeFeeRates = r.spotExchangeFeeRates
	}
	if r.futuresExchangeFeeRates != nil {
		futuresPosition.ExchangeFeeRates = r.futuresExchangeFeeRates
	}
	if !r.syncState.AvgFeeCost.IsZero() {
		spotPosition.FeeAverageCosts = map[string]fixedpoint.Value{
			r.syncState.FeeSymbol: r.syncState.AvgFeeCost,
		}
		futuresPosition.FeeAverageCosts = map[string]fixedpoint.Value{
			r.syncState.FeeSymbol: r.syncState.AvgFeeCost,
		}
	}

	spotProfitStats := types.NewProfitStats(spotMarket)
	futuresProfitStats := types.NewProfitStats(futuresMarket)

	spotTrades := r.spotWorker.Executor().AllTrades()
	for _, trade := range spotTrades {
		profit, netProfit, madeProfit := spotPosition.AddTrade(trade)
		if madeProfit {
			p := spotPosition.NewProfit(trade, profit, netProfit)
			spotProfitStats.AddProfit(p)
		}
	}

	futuresTrades := r.futuresWorker.Executor().AllTrades()
	for _, trade := range futuresTrades {
		profit, netProfit, madeProfit := futuresPosition.AddTrade(trade)
		if madeProfit {
			p := futuresPosition.NewProfit(trade, profit, netProfit)
			futuresProfitStats.AddProfit(p)
		}
	}

	return RoundPnL{
		FundingIncome:      fundingIncome,
		SpotProfitStats:    spotProfitStats,
		FuturesProfitStats: futuresProfitStats,
	}
}
