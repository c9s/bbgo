package accounting

import (
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo/types"
)

type ProfitAndLossCalculator struct {
	Symbol             string
	StartTime          time.Time
	CurrentPrice       float64
	Trades             []types.Trade
	TradingFeeCurrency string
}

func (c *ProfitAndLossCalculator) AddTrade(trade types.Trade) {
	c.Trades = append(c.Trades, trade)
}

func (c *ProfitAndLossCalculator) SetCurrentPrice(price float64) {
	c.CurrentPrice = price
}

func (c *ProfitAndLossCalculator) Calculate() *ProfitAndLossReport {
	// copy trades, so that we can truncate it.
	var trades = c.Trades
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
		stockFee := c.CurrentPrice * stock * feeRate
		unrealizedProfit += (c.CurrentPrice-averageCost)*stock - stockFee
	}

	return &ProfitAndLossReport{
		Symbol:       c.Symbol,
		StartTime:    c.StartTime,
		CurrentPrice: c.CurrentPrice,
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
