package bbgo

import (
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

func CalculateAverageCost(trades []types.Trade) (averageCost float64) {
	var totalCost = 0.0
	var totalQuantity = 0.0
	for _, t := range trades {
		if t.IsBuyer {
			totalCost += t.Price * t.Volume
			totalQuantity += t.Volume
		} else {
			totalCost -= t.Price * t.Volume
			totalQuantity -= t.Volume
		}
	}

	averageCost = totalCost / totalQuantity
	return
}

type ProfitAndLossCalculator struct {
	Symbol       string
	StartTime    time.Time
	CurrentPrice float64
	Trades       []types.Trade

	CurrencyPrice map[string]float64
}

func (c *ProfitAndLossCalculator) AddTrade(trade types.Trade) {
	c.Trades = append(c.Trades, trade)
}

func (c *ProfitAndLossCalculator) SetCurrencyPrice(symbol string, price float64) {
	if c.CurrencyPrice == nil {
		c.CurrencyPrice = map[string]float64{}
	}

	c.CurrencyPrice[symbol] = price
}

func (c *ProfitAndLossCalculator) SetCurrentPrice(price float64) {
	c.CurrentPrice = price
}

func (c *ProfitAndLossCalculator) Calculate() *ProfitAndLossReport {
	// copy trades, so that we can truncate it.
	var trades = c.Trades

	var bidVolume = 0.0
	var bidAmount = 0.0
	var bidFee = 0.0

	// find the first buy trade
	var firstBidIndex = -1
	for idx, t := range trades {
		if t.IsBuyer {
			firstBidIndex = idx
			break
		}
	}
	if firstBidIndex > 0 {
		trades = trades[firstBidIndex:]
	}

	for _, t := range trades {
		if t.IsBuyer {
			bidVolume += t.Volume
			bidAmount += t.Price * t.Volume

			// since we use USDT as the quote currency, we simply check if it matches the currency symbol
			if strings.HasPrefix(t.Symbol, t.FeeCurrency) {
				bidFee += t.Price * t.Fee
			} else if t.FeeCurrency == "USDT" {
				bidFee += t.Fee
			}
		}
	}

	log.Infof("average bid price = (total amount %f + total fee %f) / volume %f", bidAmount, bidFee, bidVolume)
	profit := 0.0
	averageBidPrice := (bidAmount + bidFee) / bidVolume

	var feeRate = 0.001
	var askVolume = 0.0
	var askFee = 0.0
	for _, t := range trades {
		if !t.IsBuyer {
			profit += (t.Price - averageBidPrice) * t.Volume
			askVolume += t.Volume
			switch t.FeeCurrency {
			case "USDT":
				askFee += t.Fee
			}
		}
	}

	profit -= askFee

	stock := bidVolume - askVolume
	futureFee := 0.0
	if stock > 0 {
		stockFee := c.CurrentPrice * feeRate * stock
		profit += (c.CurrentPrice-averageBidPrice)*stock - stockFee
		futureFee += stockFee
	}

	fee := bidFee + askFee + futureFee

	return &ProfitAndLossReport{
		Symbol:       c.Symbol,
		StartTime:    c.StartTime,
		CurrentPrice: c.CurrentPrice,
		NumTrades:    len(trades),

		Profit:          profit,
		AverageBidPrice: averageBidPrice,
		Stock:           stock,
		Fee:             fee,
	}
}

type ProfitAndLossReport struct {
	CurrentPrice float64
	StartTime    time.Time
	Symbol       string

	NumTrades       int
	Profit          float64
	AverageBidPrice float64
	Stock           float64
	Fee             float64
}

func (report ProfitAndLossReport) Print() {
	log.Infof("trades since: %v", report.StartTime)
	log.Infof("average bid price: %s", USD.FormatMoneyFloat64(report.AverageBidPrice))
	log.Infof("stock volume: %f", report.Stock)
	log.Infof("current price: %s", USD.FormatMoneyFloat64(report.CurrentPrice))
	log.Infof("overall profit: %s", USD.FormatMoneyFloat64(report.Profit))
}

