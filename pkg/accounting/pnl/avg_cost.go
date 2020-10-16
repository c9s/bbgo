package pnl

import (
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/accounting"
	"github.com/c9s/bbgo/pkg/types"
)

type AverageCostCalculator struct {
	Symbol             string
	StartTime          time.Time
	TradingFeeCurrency string
}

func (c *AverageCostCalculator) Calculate(trades []types.Trade, currentPrice float64) *accounting.ProfitAndLossReport {
	// copy trades, so that we can truncate it.
	var bidVolume = 0.0
	var bidAmount = 0.0

	var askVolume = 0.0

	var feeUSD = 0.0
	var bidFeeUSD = 0.0
	var feeRate = 0.0015

	var currencyFees = map[string]float64{}

	for _, trade := range trades {
		if trade.Symbol == c.Symbol {
			if trade.IsBuyer {
				bidVolume += trade.Quantity
				bidAmount += trade.Price * trade.Quantity
			}

			// since we use USDT as the quote currency, we simply check if it matches the currency symbol
			if strings.HasPrefix(trade.Symbol, trade.FeeCurrency) {
				bidVolume -= trade.Fee
				feeUSD += trade.Price * trade.Fee
				if trade.IsBuyer {
					bidFeeUSD += trade.Price * trade.Fee
				}
			} else if trade.FeeCurrency == "USDT" {
				feeUSD += trade.Fee
				if trade.IsBuyer {
					bidFeeUSD += trade.Fee
				}
			}

		} else {
			if trade.FeeCurrency == c.TradingFeeCurrency {
				bidVolume -= trade.Fee
			}
		}

		if _, ok := currencyFees[trade.FeeCurrency]; !ok {
			currencyFees[trade.FeeCurrency] = 0.0
		}
		currencyFees[trade.FeeCurrency] += trade.Fee
	}

	logrus.Infof("average bid price = (total amount %f + total feeUSD %f) / volume %f", bidAmount, bidFeeUSD, bidVolume)
	profit := 0.0
	averageCost := (bidAmount + bidFeeUSD) / bidVolume

	for _, t := range trades {
		if t.Symbol != c.Symbol {
			continue
		}

		if t.IsBuyer {
			continue
		}

		profit += (t.Price - averageCost) * t.Quantity
		askVolume += t.Quantity
	}

	profit -= feeUSD
	unrealizedProfit := profit

	stock := bidVolume - askVolume
	if stock > 0 {
		stockFee := currentPrice * stock * feeRate
		unrealizedProfit += (currentPrice-averageCost)*stock - stockFee
	}

	return &accounting.ProfitAndLossReport{
		Symbol:       c.Symbol,
		StartTime:    c.StartTime,
		CurrentPrice: currentPrice,
		NumTrades:    len(trades),

		BidVolume: bidVolume,
		AskVolume: askVolume,

		Stock:            stock,
		Profit:           profit,
		UnrealizedProfit: unrealizedProfit,
		AverageBidCost:   averageCost,
		FeeUSD:           feeUSD,
		CurrencyFees:     currencyFees,
	}
}
