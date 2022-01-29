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
}

func (c *AverageCostCalculator) Calculate(symbol string, trades []types.Trade, currentPrice float64) *AverageCostPnlReport {
	// copy trades, so that we can truncate it.
	var bidVolume = 0.0
	var askVolume = 0.0
	var feeUSD = 0.0

	if len(trades) == 0 {
		return &AverageCostPnlReport{
			Symbol:     symbol,
			Market:     c.Market,
			LastPrice:  currentPrice,
			NumTrades:  0,
			BuyVolume:  bidVolume,
			SellVolume: askVolume,
			FeeInUSD:   feeUSD,
		}
	}

	var currencyFees = map[string]float64{}

	var position = types.NewPositionFromMarket(c.Market)
	position.SetFeeRate(types.ExchangeFee{
		// binance vip 0 uses 0.075%
		MakerFeeRate: fixedpoint.NewFromFloat(0.075 * 0.01),
		TakerFeeRate: fixedpoint.NewFromFloat(0.075 * 0.01),
	})

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
			totalProfit += profit
			totalNetProfit += netProfit
		}

		if trade.IsBuyer {
			bidVolume += trade.Quantity
		} else {
			askVolume += trade.Quantity
		}

		if _, ok := currencyFees[trade.FeeCurrency]; !ok {
			currencyFees[trade.FeeCurrency] = trade.Fee
		} else {
			currencyFees[trade.FeeCurrency] += trade.Fee
		}

		tradeIDs[trade.ID] = trade
	}

	unrealizedProfit := (fixedpoint.NewFromFloat(currentPrice) - position.AverageCost).Mul(position.GetBase())
	return &AverageCostPnlReport{
		Symbol:    symbol,
		Market:    c.Market,
		LastPrice: currentPrice,
		NumTrades: len(trades),
		StartTime: time.Time(trades[0].Time),

		BuyVolume:  bidVolume,
		SellVolume: askVolume,

		Stock:            position.GetBase().Float64(),
		Profit:           totalProfit,
		NetProfit:        totalNetProfit,
		UnrealizedProfit: unrealizedProfit,
		AverageCost:      position.AverageCost.Float64(),
		FeeInUSD:         (totalProfit - totalNetProfit).Float64(),
		CurrencyFees:     currencyFees,
	}
}
