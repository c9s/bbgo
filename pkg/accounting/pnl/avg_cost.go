package pnl

import (
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type AverageCostCalculator struct {
	TradingFeeCurrency string
	Market             types.Market
	ExchangeFee        *types.ExchangeFee
}

func (c *AverageCostCalculator) Calculate(symbol string, trades []types.Trade, currentPrice fixedpoint.Value) *AverageCostPnLReport {
	// copy trades, so that we can truncate it.
	var bidVolume = fixedpoint.Zero
	var askVolume = fixedpoint.Zero
	var feeUSD = fixedpoint.Zero
	var grossProfit = fixedpoint.Zero
	var grossLoss = fixedpoint.Zero

	var position = types.NewPositionFromMarket(c.Market)

	if c.ExchangeFee != nil {
		position.SetFeeRate(*c.ExchangeFee)
	} else {
		makerFeeRate := 0.075 * 0.01

		if c.Market.QuoteCurrency == "BUSD" {
			makerFeeRate = 0
		}

		position.SetFeeRate(types.ExchangeFee{
			// binance vip 0 uses 0.075%
			MakerFeeRate: fixedpoint.NewFromFloat(makerFeeRate),
			TakerFeeRate: fixedpoint.NewFromFloat(0.075 * 0.01),
		})
	}

	if len(trades) == 0 {
		return &AverageCostPnLReport{
			Symbol:     symbol,
			Market:     c.Market,
			LastPrice:  currentPrice,
			NumTrades:  0,
			Position:   position,
			BuyVolume:  bidVolume,
			SellVolume: askVolume,
			FeeInUSD:   feeUSD,
		}
	}

	var currencyFees = map[string]fixedpoint.Value{}

	// TODO: configure the exchange fee rate here later
	// position.SetExchangeFeeRate()
	var totalProfit fixedpoint.Value
	var totalNetProfit fixedpoint.Value

	var tradeIDs = map[uint64]types.Trade{}

	for _, trade := range trades {
		if _, exists := tradeIDs[trade.ID]; exists {
			log.Warnf("duplicated trade: %+v", trade)
			continue
		}

		if trade.Symbol != symbol {
			continue
		}

		profit, netProfit, madeProfit := position.AddTrade(trade)
		if madeProfit {
			totalProfit = totalProfit.Add(profit)
			totalNetProfit = totalNetProfit.Add(netProfit)
		}

		if profit.Sign() > 0 {
			grossProfit = grossProfit.Add(profit)
		} else if profit.Sign() < 0 {
			grossLoss = grossLoss.Add(profit)
		}

		if trade.IsBuyer {
			bidVolume = bidVolume.Add(trade.Quantity)
		} else {
			askVolume = askVolume.Add(trade.Quantity)
		}

		if _, ok := currencyFees[trade.FeeCurrency]; !ok {
			currencyFees[trade.FeeCurrency] = trade.Fee
		} else {
			currencyFees[trade.FeeCurrency] = currencyFees[trade.FeeCurrency].Add(trade.Fee)
		}

		tradeIDs[trade.ID] = trade
	}

	unrealizedProfit := currentPrice.Sub(position.AverageCost).
		Mul(position.GetBase())

	return &AverageCostPnLReport{
		Symbol:    symbol,
		Market:    c.Market,
		LastPrice: currentPrice,
		NumTrades: len(trades),
		StartTime: time.Time(trades[0].Time),
		Position:  position,

		BuyVolume:  bidVolume,
		SellVolume: askVolume,

		BaseAssetPosition: position.GetBase(),
		Profit:            totalProfit,
		NetProfit:         totalNetProfit,
		UnrealizedProfit:  unrealizedProfit,

		GrossProfit: grossProfit,
		GrossLoss:   grossLoss,

		AverageCost:  position.AverageCost,
		FeeInUSD:     totalProfit.Sub(totalNetProfit),
		CurrencyFees: currencyFees,
	}
}
