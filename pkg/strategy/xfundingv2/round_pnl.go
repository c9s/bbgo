package xfundingv2

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/style"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/slack-go/slack"
)

type RoundRealizedPnL struct {
	FundingIncome      fixedpoint.Value
	SpotProfitStats    *types.ProfitStats
	FuturesProfitStats *types.ProfitStats
	SpotPosition       *types.Position
	FuturesPosition    *types.Position
}

func (p *RoundRealizedPnL) SlackAttachment() slack.Attachment {
	totalNetPnL := p.FundingIncome.Add(
		p.NetPnL(),
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

func (p *RoundRealizedPnL) NetPnL() fixedpoint.Value {
	return p.SpotProfitStats.AccumulatedNetProfit.Add(
		p.FuturesProfitStats.AccumulatedNetProfit,
	)
}

func (p *RoundRealizedPnL) TotalPnL() fixedpoint.Value {
	return p.FundingIncome.Add(
		p.NetPnL(),
	)
}

func (r *ArbitrageRound) RealizedPnL() *RoundRealizedPnL {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.realizedPnL()
}

func (r *ArbitrageRound) realizedPnL() *RoundRealizedPnL {
	fundingIncome := r.totalFundingIncome()

	spotMarket := r.spotWorker.Market()
	futuresMarket := r.futuresWorker.Market()

	spotPosition := types.NewPositionFromMarket(spotMarket)
	spotPosition.UseExcludeFeeFromCostMode()
	futuresPosition := types.NewPositionFromMarket(futuresMarket)
	futuresPosition.UseExcludeFeeFromCostMode()
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

	roundPnL := RoundRealizedPnL{
		FundingIncome:      fundingIncome,
		SpotProfitStats:    spotProfitStats,
		FuturesProfitStats: futuresProfitStats,
		SpotPosition:       spotPosition,
		FuturesPosition:    futuresPosition,
	}
	return &roundPnL
}

type RoundUnrealizedPnL struct {
	RoundRealizedPnL

	// prices used to calculate the unrealized PnL
	SpotPrice    fixedpoint.Value
	FuturesPrice fixedpoint.Value
	// unrealized PnL of the open positions
	UnrealizedSpotPnL    fixedpoint.Value
	UnrealizedFuturesPnL fixedpoint.Value
}

// UnrealizedPnL calculates the unrealized profit and loss of the round by the given mark prices
func (r *ArbitrageRound) UnrealizedPnL(spotPrice, futuresPrice fixedpoint.Value) *RoundUnrealizedPnL {
	r.mu.Lock()
	defer r.mu.Unlock()

	realized := r.realizedPnL()

	result := &RoundUnrealizedPnL{
		RoundRealizedPnL: *realized,
	}

	// calculate estimated fees based on the current prices and base position sizes
	spotFeeRate := r.spotExchangeFeeRates[r.syncState.SpotExchangeName].TakerFeeRate
	futuresFeeRate := r.futuresExchangeFeeRates[r.syncState.FuturesExchangeName].TakerFeeRate
	estimatedSpotFee := spotPrice.Mul(
		realized.SpotPosition.Base.Abs(),
	).Mul(spotFeeRate)
	estimatedFuturesFee := futuresPrice.Mul(
		realized.FuturesPosition.Base.Abs(),
	).Mul(futuresFeeRate)

	spotUnrealizedPnL := realized.SpotPosition.UnrealizedProfit(spotPrice)
	result.SpotPrice = spotPrice
	result.UnrealizedSpotPnL = spotUnrealizedPnL.Sub(estimatedSpotFee)
	futuresUnrealizedPnL := realized.FuturesPosition.UnrealizedProfit(futuresPrice)
	result.FuturesPrice = futuresPrice
	result.UnrealizedFuturesPnL = futuresUnrealizedPnL.Sub(estimatedFuturesFee)

	return result
}

func (r *RoundUnrealizedPnL) TotalSpotNetPnL() fixedpoint.Value {
	return r.SpotProfitStats.AccumulatedNetProfit.Add(r.UnrealizedSpotPnL)
}

func (r *RoundUnrealizedPnL) TotalFuturesNetPnL() fixedpoint.Value {
	return r.FuturesProfitStats.AccumulatedNetProfit.Add(r.UnrealizedFuturesPnL)
}

// TotalPnL returns realized net PnL (spot + futures + realized funding income) plus the current
// unrealized PnL of both open legs.
func (p *RoundUnrealizedPnL) TotalPnL() fixedpoint.Value {
	return p.RoundRealizedPnL.TotalPnL().Add(p.UnrealizedSpotPnL).Add(p.UnrealizedFuturesPnL)
}
